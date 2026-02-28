package import_manager

import (
	"database/sql"
	"errors"
	"os"
	"strings"
)

const (
	ErrorCategoryIO      = "io"
	ErrorCategoryParse   = "parse"
	ErrorCategoryDB      = "db"
	ErrorCategoryUnknown = "unknown"
)

func categorizeError(err error) string {
	if err == nil {
		return ErrorCategoryUnknown
	}

	if errors.Is(err, sql.ErrNoRows) {
		return ErrorCategoryDB
	}

	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return ErrorCategoryIO
	}

	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "exif") || strings.Contains(errMsg, "filename") || strings.Contains(errMsg, "parse") || strings.Contains(errMsg, "format") {
		return ErrorCategoryParse
	}
	if strings.Contains(errMsg, "sqlite") || strings.Contains(errMsg, "database") || strings.Contains(errMsg, "constraint") || strings.Contains(errMsg, "transaction") {
		return ErrorCategoryDB
	}
	if strings.Contains(errMsg, "no such file") || strings.Contains(errMsg, "permission") || strings.Contains(errMsg, "input/output") {
		return ErrorCategoryIO
	}

	return ErrorCategoryUnknown
}
