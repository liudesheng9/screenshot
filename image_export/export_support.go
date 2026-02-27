package image_export

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

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
