// Package import_manager imports screenshot metadata from external PNG
// directories into the local screenshots database.
package import_manager

import (
	"database/sql"
	"fmt"
	"log"

	"screenshot_server/image_manipulation"
)

const (
	defaultBatchSize   = 100
	defaultWorkerCount = 4
)

type ImageMeta = image_manipulation.ImageMeta

type ImportConfig struct {
	DB               *sql.DB
	Directory        string
	Remap            map[int]int
	BatchSize        int
	WorkerCount      int
	ProgressChan     chan<- ImportProgress
	ProgressCallback func(ImportProgress)
	Logger           *log.Logger
}

type ImportProgress struct {
	Processed int
	Total     int
	Inserted  int
	Updated   int
	Skipped   int
	Failed    int
}

type ImportResult struct {
	Processed         int
	Total             int
	Inserted          int
	Updated           int
	Skipped           int
	Failed            int
	FailedFiles       []string
	ErrorsByCategory  map[string]int
	Interrupted       bool
	BatchFallbackUsed int
}

func (r ImportResult) Summary() string {
	return fmt.Sprintf(
		"processed=%d/%d inserted=%d updated=%d skipped=%d failed=%d",
		r.Processed,
		r.Total,
		r.Inserted,
		r.Updated,
		r.Skipped,
		r.Failed,
	)
}

type ImportBatchResult struct {
	Processed        int
	Inserted         int
	Updated          int
	Skipped          int
	Failed           int
	FailedFiles      []string
	ErrorsByCategory map[string]int
	FallbackUsed     bool
}

type importRecord struct {
	FilePath string
	FileName string
	FileID   string
	Meta     ImageMeta
}

type importRecordResult struct {
	record importRecord
	err    error
	file   string
}
