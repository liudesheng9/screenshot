package image_export

import (
	"bytes"
	"database/sql"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultDumpDir           = "./img_dump"
	defaultJPEGQuality       = 85
	defaultIOWorkers         = 2
	defaultProcessingWorkers = 10
	bufferedQueueCapacity    = 10
	jpegFileExt              = ".jpg"
)

type BufferedImage struct {
	SourcePath string
	TargetPath string
	Data       []byte
}

type imageTransformTask struct {
	sourcePath string
	targetPath string
}

type imageTransformResult struct {
	err error
}

type TimeRange struct {
	Date        string
	Year        int
	Month       int
	Day         int
	StartMinute int
	EndMinute   int
}

type CopyResult struct {
	Archived int
	Existing int
	Missing  int
	Copied   int
	Skipped  int
	Failed   int
}

type ProgressConfig struct {
	EveryImages   int
	EveryInterval time.Duration
}

var defaultProgressConfig = ProgressConfig{
	EveryImages:   5,
	EveryInterval: time.Second,
}

func (r CopyResult) Summary() string {
	return fmt.Sprintf(
		"archived=%d exist=%d missing=%d copied=%d skipped=%d failed=%d",
		r.Archived,
		r.Existing,
		r.Missing,
		r.Copied,
		r.Skipped,
		r.Failed,
	)
}

type CountResult struct {
	Archived int
	Existing int
	Missing  int
}

func (r CountResult) Summary() string {
	return fmt.Sprintf("archived=%d exist=%d missing=%d", r.Archived, r.Existing, r.Missing)
}

func ParseRange(input string) (TimeRange, error) {
	rangeStr := strings.TrimSpace(input)
	if len(rangeStr) != 17 || rangeStr[12] != '-' {
		return TimeRange{}, fmt.Errorf("invalid range format, expected YYYYMMDDHHMM-HHMM")
	}

	dateStr := rangeStr[:8]
	startStr := rangeStr[8:12]
	endStr := rangeStr[13:17]

	if !isDigits(dateStr) || !isDigits(startStr) || !isDigits(endStr) {
		return TimeRange{}, fmt.Errorf("invalid range format, expected numeric YYYYMMDDHHMM-HHMM")
	}

	year, _ := strconv.Atoi(dateStr[:4])
	month, _ := strconv.Atoi(dateStr[4:6])
	day, _ := strconv.Atoi(dateStr[6:8])
	if !isValidDate(year, month, day) {
		return TimeRange{}, fmt.Errorf("invalid date in range: %s", dateStr)
	}

	startHour, startMinute, err := parseHHMM(startStr)
	if err != nil {
		return TimeRange{}, fmt.Errorf("invalid start time: %w", err)
	}
	endHour, endMinute, err := parseHHMM(endStr)
	if err != nil {
		return TimeRange{}, fmt.Errorf("invalid end time: %w", err)
	}

	startTotal := startHour*60 + startMinute
	endTotal := endHour*60 + endMinute
	if startTotal > endTotal {
		return TimeRange{}, fmt.Errorf("start time must be <= end time")
	}

	return TimeRange{
		Date:        dateStr,
		Year:        year,
		Month:       month,
		Day:         day,
		StartMinute: startTotal,
		EndMinute:   endTotal,
	}, nil
}

func CountImages(db *sql.DB, imgPath string, tr TimeRange) (CountResult, error) {
	archived, existing, missing, err := collectExistingFiles(db, imgPath, tr)
	if err != nil {
		return CountResult{}, err
	}
	return CountResult{
		Archived: archived,
		Existing: len(existing),
		Missing:  missing,
	}, nil
}

func CopyImages(db *sql.DB, imgPath, dest string, tr TimeRange) (CopyResult, error) {
	return copyImages(db, imgPath, dest, tr, nil, defaultProgressConfig)
}

func CopyImagesWithProgress(
	db *sql.DB,
	imgPath, dest string,
	tr TimeRange,
	progress chan<- ProgressUpdate,
) (CopyResult, error) {
	defer closeProgressChannel(progress)
	return copyImages(db, imgPath, dest, tr, progress, defaultProgressConfig)
}

func copyImages(
	db *sql.DB,
	imgPath, dest string,
	tr TimeRange,
	progress chan<- ProgressUpdate,
	progressConfig ProgressConfig,
) (CopyResult, error) {
	result := CopyResult{}

	archived, paths, missing, err := collectExistingFiles(db, imgPath, tr)
	if err != nil {
		return result, err
	}
	result.Archived = archived
	result.Existing = len(paths)
	result.Missing = missing

	destPath := resolveDestPath(dest)
	if err := validateDestPath(imgPath, destPath); err != nil {
		return result, err
	}
	if err := os.MkdirAll(destPath, os.ModePerm); err != nil {
		return result, fmt.Errorf("failed to create dest directory: %w", err)
	}
	if err := clearDirectoryContents(destPath); err != nil {
		return result, fmt.Errorf("failed to clear dest directory: %w", err)
	}

	transformTasks, skipped, failed := buildImageTransformTasks(paths, destPath)
	result.Skipped += skipped
	result.Failed += failed

	copied, convertFailed := processImageTransformQueue(
		transformTasks,
		defaultProcessingWorkers,
		defaultJPEGQuality,
		progress,
		progressConfig,
	)
	result.Copied += copied
	result.Failed += convertFailed

	return result, nil
}

func buildImageTransformTasks(paths []string, destPath string) ([]imageTransformTask, int, int) {
	usedTargets := make(map[string]struct{}, len(paths))
	tasks := make([]imageTransformTask, 0, len(paths))
	skipped := 0
	failed := 0

	for _, src := range paths {
		targetName, err := toJPEGFileName(filepath.Base(src))
		if err != nil {
			failed++
			continue
		}
		if _, exists := usedTargets[targetName]; exists {
			skipped++
			continue
		}
		usedTargets[targetName] = struct{}{}

		tasks = append(tasks, imageTransformTask{
			sourcePath: src,
			targetPath: filepath.Join(destPath, targetName),
		})
	}
	return tasks, skipped, failed
}

func processImageTransformQueue(
	tasks []imageTransformTask,
	workerCount,
	jpegQuality int,
	progress chan<- ProgressUpdate,
	progressConfig ProgressConfig,
) (int, int) {
	if len(tasks) == 0 {
		return 0, 0
	}

	ioWorkerTotal := normalizeWorkerCount(defaultIOWorkers)
	processingWorkerTotal := normalizeWorkerCount(workerCount)
	progressConfig = normalizeProgressConfig(progressConfig)
	imageTransformTaskQueue := make(chan imageTransformTask, len(tasks))
	bufferedImageQueue := make(chan *BufferedImage, bufferedQueueCapacity)
	resultQueue := make(chan imageTransformResult, len(tasks))
	stats := NewPipelineWorkerStats(ioWorkerTotal, processingWorkerTotal)
	defer stats.Close()

	var ioWorkers sync.WaitGroup
	ioWorkers.Add(ioWorkerTotal)
	for i := 0; i < ioWorkerTotal; i++ {
		workerID := i
		go func() {
			defer ioWorkers.Done()
			runIOWorker(workerID, imageTransformTaskQueue, bufferedImageQueue, resultQueue, stats)
		}()
	}

	var processingWorkers sync.WaitGroup
	processingWorkers.Add(processingWorkerTotal)
	for i := 0; i < processingWorkerTotal; i++ {
		workerID := i
		go func() {
			defer processingWorkers.Done()
			runImageTransformWorker(workerID, bufferedImageQueue, resultQueue, jpegQuality, stats)
		}()
	}

	go func() {
		for _, task := range tasks {
			imageTransformTaskQueue <- task
		}
		close(imageTransformTaskQueue)
	}()

	go func() {
		ioWorkers.Wait()
		close(bufferedImageQueue)
	}()

	go func() {
		ioWorkers.Wait()
		processingWorkers.Wait()
		close(resultQueue)
	}()

	copied := 0
	failed := 0
	completedSinceUpdate := 0

	sendProgress := func() {
		if progress == nil {
			return
		}
		update := ProgressUpdate{
			WorkerCounts: stats.GetWorkerCounts(),
			WorkerTasks:  stats.GetWorkerTasks(),
			WorkerRefs:   stats.GetWorkerRefs(),
			Total:        copied,
			Target:       len(tasks),
			Timestamp:    time.Now(),
		}
		select {
		case progress <- update:
		default:
		}
	}

	var ticker *time.Ticker
	var tickerChan <-chan time.Time
	if progress != nil && progressConfig.EveryInterval > 0 {
		ticker = time.NewTicker(progressConfig.EveryInterval)
		tickerChan = ticker.C
		defer ticker.Stop()
	}

	for {
		select {
		case transformResult, ok := <-resultQueue:
			if !ok {
				sendProgress()
				return copied, failed
			}
			if transformResult.err != nil {
				failed++
				continue
			}
			copied++
			completedSinceUpdate++
			if progress != nil && progressConfig.EveryImages > 0 && completedSinceUpdate >= progressConfig.EveryImages {
				sendProgress()
				completedSinceUpdate = 0
			}
		case <-tickerChan:
			sendProgress()
			completedSinceUpdate = 0
		}
	}
}

func runImageTransformWorker(
	workerID int,
	bufferedImageQueue <-chan *BufferedImage,
	resultQueue chan<- imageTransformResult,
	jpegQuality int,
	stats *WorkerStats,
) {
	internalWorkerID := processingWorkerInternalID(workerID)
	for bufferedImage := range bufferedImageQueue {
		err := convertImageToJPEG(bufferedImage, jpegQuality, internalWorkerID, stats)
		if bufferedImage != nil {
			bufferedImage.Data = nil
		}
		if err == nil && stats != nil {
			stats.Report(internalWorkerID)
		}
		resultQueue <- imageTransformResult{err: err}
	}
}

func runIOWorker(
	workerID int,
	taskQueue <-chan imageTransformTask,
	bufferedImageQueue chan<- *BufferedImage,
	resultQueue chan<- imageTransformResult,
	stats *WorkerStats,
) {
	internalWorkerID := ioWorkerInternalID(workerID)
	for task := range taskQueue {
		if stats != nil {
			stats.ReportStage(internalWorkerID, filepath.Base(task.sourcePath), StageReading)
		}

		data, err := os.ReadFile(task.sourcePath)
		if err != nil {
			if stats != nil {
				stats.ClearWorkerTask(internalWorkerID)
			}
			resultQueue <- imageTransformResult{err: fmt.Errorf("read source image %s: %w", task.sourcePath, err)}
			continue
		}

		bufferedImageQueue <- &BufferedImage{
			SourcePath: task.sourcePath,
			TargetPath: task.targetPath,
			Data:       data,
		}

		if stats != nil {
			stats.Report(internalWorkerID)
			stats.ClearWorkerTask(internalWorkerID)
		}
	}
}

func normalizeProgressConfig(config ProgressConfig) ProgressConfig {
	if config.EveryImages < 1 {
		config.EveryImages = defaultProgressConfig.EveryImages
	}
	if config.EveryInterval <= 0 {
		config.EveryInterval = defaultProgressConfig.EveryInterval
	}
	return config
}

func closeProgressChannel(progress chan<- ProgressUpdate) {
	if progress != nil {
		close(progress)
	}
}

func normalizeWorkerCount(workerCount int) int {
	if workerCount < 1 {
		workerCount = 1
	}
	return workerCount
}

func convertImageToJPEG(bufferedImage *BufferedImage, quality int, workerID int, reporter StageReporter) error {
	if bufferedImage == nil {
		return fmt.Errorf("buffered image is nil")
	}

	srcPath := bufferedImage.SourcePath
	destPath := bufferedImage.TargetPath
	if strings.TrimSpace(srcPath) == "" {
		return fmt.Errorf("source image path is empty")
	}
	if strings.TrimSpace(destPath) == "" {
		return fmt.Errorf("target image path is empty")
	}
	if len(bufferedImage.Data) == 0 {
		return fmt.Errorf("source image %s has no data", srcPath)
	}

	if quality < 1 || quality > 100 {
		quality = defaultJPEGQuality
	}

	if reporter != nil {
		defer reporter.ClearWorkerTask(workerID)
	}

	reportStage := func(stage Stage) {
		if reporter == nil {
			return
		}
		reporter.ReportStage(workerID, filepath.Base(srcPath), stage)
	}

	reportStage(StageDecode)
	srcImage, _, err := image.Decode(bytes.NewReader(bufferedImage.Data))
	if err != nil {
		return fmt.Errorf("decode source image %s: %w", srcPath, err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(destPath), ".img_export_*.jpg")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", destPath, err)
	}
	tmpPath := tmpFile.Name()
	keepTemp := false
	tmpFileClosed := false
	defer func() {
		if !tmpFileClosed {
			_ = tmpFile.Close()
		}
		if !keepTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	reportStage(StageEncode)
	if err := jpeg.Encode(tmpFile, srcImage, &jpeg.Options{Quality: quality}); err != nil {
		return fmt.Errorf("encode jpeg %s: %w", destPath, err)
	}
	reportStage(StageWrite)
	reportStage(StageSync)
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync jpeg %s: %w", destPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close jpeg %s: %w", destPath, err)
	}
	tmpFileClosed = true
	keepTemp = true

	if err := os.Rename(tmpPath, destPath); err != nil {
		keepTemp = false
		return fmt.Errorf("move jpeg into place %s: %w", destPath, err)
	}
	return nil
}
