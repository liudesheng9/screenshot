package image_export

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"screenshot_server/utils"
)

const defaultDumpDir = "./img_dump"

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

	destPath := strings.TrimSpace(dest)
	if destPath == "" {
		destPath = defaultDumpDir
	}
	destPath = filepath.Clean(destPath)
	if err := os.MkdirAll(destPath, os.ModePerm); err != nil {
		return result, fmt.Errorf("failed to create dest directory: %w", err)
	}

	for _, src := range paths {
		base := filepath.Base(src)
		target := filepath.Join(destPath, base)
		if _, err := os.Stat(target); err == nil {
			result.Skipped++
			continue
		} else if !os.IsNotExist(err) {
			result.Failed++
			continue
		}

		if err := utils.Copy_file(src, target); err != nil {
			result.Failed++
			continue
		}
		result.Copied++
	}

	return result, nil
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
		full := filepath.Join(imgPath, name)
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
