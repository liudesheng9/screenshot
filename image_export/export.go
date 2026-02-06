package image_export

import (
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
	defaultDumpDir     = "./img_dump"
	defaultJPEGQuality = 85
	defaultWorkers     = 10
	jpegFileExt        = ".jpg"
)

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

	copied, convertFailed := processImageTransformQueue(transformTasks, defaultWorkers, defaultJPEGQuality)
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

func processImageTransformQueue(tasks []imageTransformTask, workerCount, jpegQuality int) (int, int) {
	if len(tasks) == 0 {
		return 0, 0
	}

	workerTotal := normalizeWorkerCount(workerCount)
	taskQueue := make(chan imageTransformTask, workerTotal)
	resultQueue := make(chan imageTransformResult, len(tasks))

	var workers sync.WaitGroup
	workers.Add(workerTotal)
	for i := 0; i < workerTotal; i++ {
		go func() {
			defer workers.Done()
			runImageTransformWorker(taskQueue, resultQueue, jpegQuality)
		}()
	}

	go func() {
		for _, task := range tasks {
			taskQueue <- task
		}
		close(taskQueue)
	}()

	go func() {
		workers.Wait()
		close(resultQueue)
	}()

	copied := 0
	failed := 0
	for transformResult := range resultQueue {
		if transformResult.err != nil {
			failed++
			continue
		}
		copied++
	}
	return copied, failed
}

func runImageTransformWorker(
	taskQueue <-chan imageTransformTask,
	resultQueue chan<- imageTransformResult,
	jpegQuality int,
) {
	for task := range taskQueue {
		err := convertImageToJPEG(task.sourcePath, task.targetPath, jpegQuality)
		resultQueue <- imageTransformResult{err: err}
	}
}

func normalizeWorkerCount(workerCount int) int {
	if workerCount < 1 {
		workerCount = 1
	}
	return workerCount
}

func queryMatchingFileNames(db *sql.DB, tr TimeRange) ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	query := `
		SELECT DISTINCT file_name
		FROM screenshots
		WHERE file_name IS NOT NULL
		  AND TRIM(file_name) != ''
		  AND year = ?
		  AND month = ?
		  AND day = ?
		  AND (hour * 60 + minute) BETWEEN ? AND ?
		ORDER BY file_name
	`
	rows, err := db.Query(query, tr.Year, tr.Month, tr.Day, tr.StartMinute, tr.EndMinute)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[string]struct{})
	names := make([]string, 0)
	for rows.Next() {
		var name sql.NullString
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if !name.Valid || strings.TrimSpace(name.String) == "" {
			continue
		}
		if _, ok := seen[name.String]; ok {
			continue
		}
		seen[name.String] = struct{}{}
		names = append(names, name.String)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return names, nil
}

func queryArchiveCount(db *sql.DB, tr TimeRange) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("database is nil")
	}
	query := `
		SELECT COUNT(DISTINCT file_name)
		FROM screenshots
		WHERE file_name IS NOT NULL
		  AND TRIM(file_name) != ''
		  AND year = ?
		  AND month = ?
		  AND day = ?
		  AND (hour * 60 + minute) BETWEEN ? AND ?
	`
	var count int
	err := db.QueryRow(query, tr.Year, tr.Month, tr.Day, tr.StartMinute, tr.EndMinute).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func collectExistingFiles(db *sql.DB, imgPath string, tr TimeRange) (int, []string, int, error) {
	if err := validateImgPath(imgPath); err != nil {
		return 0, nil, 0, err
	}
	archived, err := queryArchiveCount(db, tr)
	if err != nil {
		return 0, nil, 0, err
	}
	names, err := queryMatchingFileNames(db, tr)
	if err != nil {
		return archived, nil, 0, err
	}

	existing := make([]string, 0, len(names))
	missing := 0
	for _, name := range names {
		full, err := resolvePathWithinRoot(imgPath, name)
		if err != nil {
			return archived, existing, missing, err
		}
		info, statErr := os.Stat(full)
		if statErr == nil {
			if info.IsDir() {
				missing++
				continue
			}
			existing = append(existing, full)
			continue
		}
		if os.IsNotExist(statErr) {
			missing++
			continue
		}
		return archived, existing, missing, fmt.Errorf("stat %s: %w", full, statErr)
	}
	return archived, existing, missing, nil
}

func validateImgPath(imgPath string) error {
	if strings.TrimSpace(imgPath) == "" {
		return fmt.Errorf("img_path is empty")
	}
	info, err := os.Stat(imgPath)
	if err != nil {
		return fmt.Errorf("img_path not accessible: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("img_path is not a directory")
	}
	return nil
}

func validateDestPath(imgPath, destPath string) error {
	if strings.TrimSpace(destPath) == "" {
		return fmt.Errorf("dest path is empty")
	}
	if isPathRoot(destPath) {
		return fmt.Errorf("dest path cannot be root")
	}

	absDest, err := filepath.Abs(destPath)
	if err != nil {
		return fmt.Errorf("dest path invalid: %w", err)
	}
	absImg, err := filepath.Abs(imgPath)
	if err != nil {
		return fmt.Errorf("img path invalid: %w", err)
	}
	if absDest == absImg {
		return fmt.Errorf("dest path cannot be the same as img_path")
	}
	if rel, err := filepath.Rel(absImg, absDest); err == nil {
		if rel == "." || !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != ".." {
			return fmt.Errorf("dest path cannot be inside img_path")
		}
	}
	return nil
}

func resolveDestPath(dest string) string {
	destPath := strings.TrimSpace(dest)
	if destPath == "" {
		destPath = defaultDumpDir
	}
	return filepath.Clean(destPath)
}

func toJPEGFileName(name string) (string, error) {
	baseName := strings.TrimSpace(name)
	if baseName == "" {
		return "", fmt.Errorf("empty file name")
	}
	ext := filepath.Ext(baseName)
	stem := strings.TrimSpace(strings.TrimSuffix(baseName, ext))
	if stem == "" {
		return "", fmt.Errorf("invalid file name: %s", name)
	}
	return stem + jpegFileExt, nil
}

func convertImageToJPEG(srcPath, destPath string, quality int) error {
	if quality < 1 || quality > 100 {
		quality = defaultJPEGQuality
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source image %s: %w", srcPath, err)
	}
	defer srcFile.Close()

	srcImage, _, err := image.Decode(srcFile)
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

	if err := jpeg.Encode(tmpFile, srcImage, &jpeg.Options{Quality: quality}); err != nil {
		return fmt.Errorf("encode jpeg %s: %w", destPath, err)
	}
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

func resolvePathWithinRoot(root, child string) (string, error) {
	cleanChild := filepath.Clean(strings.TrimSpace(child))
	if cleanChild == "." || cleanChild == "" {
		return "", fmt.Errorf("invalid file name: %q", child)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("invalid root path: %w", err)
	}
	full := filepath.Join(absRoot, cleanChild)
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", fmt.Errorf("invalid file path %q: %w", child, err)
	}
	rel, err := filepath.Rel(absRoot, absFull)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path %q: %w", child, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("file path escapes image directory: %q", child)
	}
	return absFull, nil
}

func clearDirectoryContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		target := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}
	return nil
}

func isPathRoot(p string) bool {
	clean := filepath.Clean(p)
	vol := filepath.VolumeName(clean)
	if vol != "" {
		rest := strings.TrimPrefix(clean, vol)
		rest = strings.TrimPrefix(rest, string(os.PathSeparator))
		return rest == ""
	}
	return clean == string(os.PathSeparator) || clean == "."
}

func parseHHMM(s string) (int, int, error) {
	if len(s) != 4 || !isDigits(s) {
		return 0, 0, fmt.Errorf("expected HHMM")
	}
	hour, _ := strconv.Atoi(s[:2])
	minute, _ := strconv.Atoi(s[2:4])
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid hour/minute")
	}
	return hour, minute, nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isValidDate(year, month, day int) bool {
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return false
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return t.Year() == year && int(t.Month()) == month && t.Day() == day
}
