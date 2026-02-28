package import_manager

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"screenshot_server/image_manipulation"
)

var filenameMetaPattern = regexp.MustCompile(`(?i)^(\d{4})(\d{2})(\d{2})_(\d{2})(\d{2})(\d{2})_(\d+)\.png$`)

func extractFromEXIF(filePath string) (ImageMeta, error) {
	return image_manipulation.Substract_Meta_from_file(filePath)
}

func extractFromFilename(filename string) (ImageMeta, error) {
	base := filepath.Base(strings.TrimSpace(filename))
	matches := filenameMetaPattern.FindStringSubmatch(base)
	if matches == nil {
		return ImageMeta{}, fmt.Errorf("filename does not match expected format: %s", base)
	}

	meta := ImageMeta{}
	var err error
	if meta.Year, err = strconv.Atoi(matches[1]); err != nil {
		return ImageMeta{}, fmt.Errorf("invalid year in filename %s: %w", base, err)
	}
	if meta.Month, err = strconv.Atoi(matches[2]); err != nil {
		return ImageMeta{}, fmt.Errorf("invalid month in filename %s: %w", base, err)
	}
	if meta.Day, err = strconv.Atoi(matches[3]); err != nil {
		return ImageMeta{}, fmt.Errorf("invalid day in filename %s: %w", base, err)
	}
	if meta.Hour, err = strconv.Atoi(matches[4]); err != nil {
		return ImageMeta{}, fmt.Errorf("invalid hour in filename %s: %w", base, err)
	}
	if meta.Minute, err = strconv.Atoi(matches[5]); err != nil {
		return ImageMeta{}, fmt.Errorf("invalid minute in filename %s: %w", base, err)
	}
	if meta.Second, err = strconv.Atoi(matches[6]); err != nil {
		return ImageMeta{}, fmt.Errorf("invalid second in filename %s: %w", base, err)
	}
	if meta.DisplayNum, err = strconv.Atoi(matches[7]); err != nil {
		return ImageMeta{}, fmt.Errorf("invalid display number in filename %s: %w", base, err)
	}

	meta.Hash = 0
	meta.HashKind = ""
	return meta, nil
}

func extractMetadata(filePath string) (ImageMeta, error) {
	meta, err := extractFromEXIF(filePath)
	if err == nil {
		return meta, nil
	}

	fallbackMeta, fallbackErr := extractFromFilename(filepath.Base(filePath))
	if fallbackErr != nil {
		return ImageMeta{}, fmt.Errorf("metadata extraction failed for %s (exif: %v, filename: %v)", filePath, err, fallbackErr)
	}
	return fallbackMeta, nil
}
