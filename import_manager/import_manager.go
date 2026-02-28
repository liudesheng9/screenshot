package import_manager

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

func ImportDirectory(config ImportConfig) (ImportResult, error) {
	config, err := normalizeImportConfig(config)
	if err != nil {
		return ImportResult{}, err
	}

	result := ImportResult{
		ErrorsByCategory: make(map[string]int),
		FailedFiles:      make([]string, 0),
	}

	interruptCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fileChan, err := discoverPNGFiles(config.Directory)
	if err != nil {
		return result, err
	}

	files := make([]string, 0, 256)
	for {
		select {
		case <-interruptCtx.Done():
			result.Interrupted = true
			return result, fmt.Errorf("import interrupted")
		case filePath, ok := <-fileChan:
			if !ok {
				goto discoverDone
			}
			files = append(files, filePath)
		}
	}

discoverDone:
	result.Total = len(files)
	reportProgress(config, importProgressFromResult(result))
	if len(files) == 0 {
		return result, nil
	}

	for start := 0; start < len(files); start += config.BatchSize {
		if interruptCtx.Err() != nil {
			result.Interrupted = true
			return result, fmt.Errorf("import interrupted")
		}

		end := start + config.BatchSize
		if end > len(files) {
			end = len(files)
		}
		batchFiles := files[start:end]

		records, prepErrors := prepareBatchRecords(batchFiles, config.Remap, config.WorkerCount)
		for _, prepErr := range prepErrors {
			category := categorizeError(prepErr.err)
			result.Processed++
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, prepErr.file)
			result.ErrorsByCategory[category]++
			config.Logger.Printf("import file=%s status=error category=%s error=%v", prepErr.file, category, prepErr.err)
			reportProgress(config, importProgressFromResult(result))
		}

		batchResult, err := processBatchRecordsWithLogger(config.DB, records, config.Logger)
		if err != nil {
			return result, err
		}

		result.Processed += batchResult.Processed
		result.Inserted += batchResult.Inserted
		result.Updated += batchResult.Updated
		result.Skipped += batchResult.Skipped
		result.Failed += batchResult.Failed
		if batchResult.FallbackUsed {
			result.BatchFallbackUsed++
		}
		if len(batchResult.FailedFiles) > 0 {
			result.FailedFiles = append(result.FailedFiles, batchResult.FailedFiles...)
		}
		for category, count := range batchResult.ErrorsByCategory {
			result.ErrorsByCategory[category] += count
		}

		reportProgress(config, importProgressFromResult(result))
	}

	return result, nil
}

func discoverPNGFiles(dir string) (<-chan string, error) {
	trimmedDir := strings.TrimSpace(dir)
	if trimmedDir == "" {
		return nil, fmt.Errorf("directory path is empty")
	}

	info, err := os.Stat(trimmedDir)
	if err != nil {
		return nil, fmt.Errorf("directory not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", trimmedDir)
	}

	out := make(chan string, 128)
	go func() {
		defer close(out)
		_ = filepath.WalkDir(trimmedDir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				log.Printf("import file=%s status=error category=%s error=%v", path, ErrorCategoryIO, walkErr)
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
				return nil
			}
			out <- path
			return nil
		})
	}()

	return out, nil
}

func processBatch(db *sql.DB, files []string, remap map[int]int) (ImportBatchResult, error) {
	if db == nil {
		return ImportBatchResult{}, fmt.Errorf("database is nil")
	}

	records, prepErrors := prepareBatchRecords(files, remap, 1)
	result := ImportBatchResult{
		ErrorsByCategory: make(map[string]int),
	}
	for _, prepErr := range prepErrors {
		category := categorizeError(prepErr.err)
		result.Processed++
		result.Failed++
		result.FailedFiles = append(result.FailedFiles, prepErr.file)
		result.ErrorsByCategory[category]++
	}

	batchResult, err := processBatchRecords(db, records)
	if err != nil {
		return result, err
	}

	result.Processed += batchResult.Processed
	result.Inserted += batchResult.Inserted
	result.Updated += batchResult.Updated
	result.Skipped += batchResult.Skipped
	result.Failed += batchResult.Failed
	result.FallbackUsed = batchResult.FallbackUsed
	if len(batchResult.FailedFiles) > 0 {
		result.FailedFiles = append(result.FailedFiles, batchResult.FailedFiles...)
	}
	for category, count := range batchResult.ErrorsByCategory {
		result.ErrorsByCategory[category] += count
	}

	return result, nil
}

func processBatchRecords(db *sql.DB, records []importRecord) (ImportBatchResult, error) {
	return processBatchRecordsWithLogger(db, records, log.Default())
}

func processBatchRecordsWithLogger(db *sql.DB, records []importRecord, logger *log.Logger) (ImportBatchResult, error) {
	if db == nil {
		return ImportBatchResult{}, fmt.Errorf("database is nil")
	}
	if logger == nil {
		logger = log.Default()
	}

	result := ImportBatchResult{
		ErrorsByCategory: make(map[string]int),
	}
	if len(records) == 0 {
		return result, nil
	}

	batchResult, err := runBatchTransaction(db, records, logger)
	if err == nil {
		return batchResult, nil
	}

	logger.Printf("import status=batch_fallback category=%s error=%v", ErrorCategoryDB, err)

	fallbackResult := ImportBatchResult{
		ErrorsByCategory: make(map[string]int),
		FallbackUsed:     true,
	}
	for _, record := range records {
		action, singleErr := processSingleRecord(db, record)
		fallbackResult.Processed++
		if singleErr != nil {
			category := categorizeError(singleErr)
			fallbackResult.Failed++
			fallbackResult.FailedFiles = append(fallbackResult.FailedFiles, record.FileName)
			fallbackResult.ErrorsByCategory[category]++
			logger.Printf("import file=%s status=error category=%s error=%v", record.FileName, category, singleErr)
			continue
		}

		switch action {
		case dedupInsert:
			fallbackResult.Inserted++
			logger.Printf("import file=%s status=success action=insert", record.FileName)
		case dedupUpdate:
			fallbackResult.Updated++
			logger.Printf("import file=%s status=success action=update", record.FileName)
		case dedupSkip:
			fallbackResult.Skipped++
			logger.Printf("import file=%s status=skip action=duplicate", record.FileName)
		}
	}

	return fallbackResult, nil
}

func runBatchTransaction(db *sql.DB, records []importRecord, logger *log.Logger) (ImportBatchResult, error) {
	result := ImportBatchResult{
		ErrorsByCategory: make(map[string]int),
	}

	tx, err := db.Begin()
	if err != nil {
		return result, err
	}

	for _, record := range records {
		action, applyErr := applyRecord(tx, record)
		if applyErr != nil {
			_ = tx.Rollback()
			category := categorizeError(applyErr)
			result.ErrorsByCategory[category]++
			logger.Printf("import file=%s status=error category=%s error=%v", record.FileName, category, applyErr)
			return result, fmt.Errorf("batch insert failed on %s: %w", record.FileName, applyErr)
		}

		result.Processed++
		switch action {
		case dedupInsert:
			result.Inserted++
			logger.Printf("import file=%s status=success action=insert", record.FileName)
		case dedupUpdate:
			result.Updated++
			logger.Printf("import file=%s status=success action=update", record.FileName)
		case dedupSkip:
			result.Skipped++
			logger.Printf("import file=%s status=skip action=duplicate", record.FileName)
		}
	}

	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		result.ErrorsByCategory[ErrorCategoryDB]++
		return result, err
	}

	return result, nil
}

func processSingleRecord(db *sql.DB, record importRecord) (dedupAction, error) {
	tx, err := db.Begin()
	if err != nil {
		return "", err
	}

	action, err := applyRecord(tx, record)
	if err != nil {
		_ = tx.Rollback()
		return "", err
	}

	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return "", err
	}
	return action, nil
}

func prepareBatchRecords(files []string, remap map[int]int, workerCount int) ([]importRecord, []importRecordResult) {
	if workerCount < 1 {
		workerCount = defaultWorkerCount
	}

	jobs := make(chan string, len(files))
	results := make(chan importRecordResult, len(files))

	var wg sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range jobs {
				record, err := prepareRecord(filePath, remap)
				if err != nil {
					results <- importRecordResult{file: filePath, err: err}
					continue
				}
				results <- importRecordResult{record: record, file: filePath}
			}
		}()
	}

	for _, filePath := range files {
		jobs <- filePath
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	records := make([]importRecord, 0, len(files))
	prepErrors := make([]importRecordResult, 0)
	for result := range results {
		if result.err != nil {
			prepErrors = append(prepErrors, result)
			continue
		}
		records = append(records, result.record)
	}

	return records, prepErrors
}

func prepareRecord(filePath string, remap map[int]int) (importRecord, error) {
	fileName := filepath.Base(filePath)
	if strings.TrimSpace(fileName) == "" {
		return importRecord{}, fmt.Errorf("invalid file path: %s", filePath)
	}

	meta, err := extractMetadata(filePath)
	if err != nil {
		return importRecord{}, err
	}
	meta.DisplayNum = applyRemap(meta.DisplayNum, remap)

	return importRecord{
		FilePath: filePath,
		FileName: fileName,
		FileID:   hashStringSHA256(fileName),
		Meta:     meta,
	}, nil
}

func normalizeImportConfig(config ImportConfig) (ImportConfig, error) {
	if config.DB == nil {
		return config, fmt.Errorf("database is nil")
	}
	config.Directory = strings.TrimSpace(config.Directory)
	if config.Directory == "" {
		return config, fmt.Errorf("directory path is empty")
	}
	if config.BatchSize < 1 {
		config.BatchSize = defaultBatchSize
	}
	if config.WorkerCount < 1 {
		config.WorkerCount = defaultWorkerCount
	}
	if config.Logger == nil {
		config.Logger = log.Default()
	}
	config.Remap = cloneRemap(config.Remap)
	return config, nil
}

func cloneRemap(remap map[int]int) map[int]int {
	if remap == nil {
		return map[int]int{}
	}
	cloned := make(map[int]int, len(remap))
	for src, dst := range remap {
		cloned[src] = dst
	}
	return cloned
}

func importProgressFromResult(result ImportResult) ImportProgress {
	return ImportProgress{
		Processed: result.Processed,
		Total:     result.Total,
		Inserted:  result.Inserted,
		Updated:   result.Updated,
		Skipped:   result.Skipped,
		Failed:    result.Failed,
	}
}

func reportProgress(config ImportConfig, progress ImportProgress) {
	if config.ProgressCallback != nil {
		config.ProgressCallback(progress)
	}
	if config.ProgressChan != nil {
		select {
		case config.ProgressChan <- progress:
		default:
		}
	}
}
