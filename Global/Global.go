package Global

import (
	"database/sql"
	"image"
	"os"
	"screenshot_server/utils"
	"sync"
	"time"
)

type StorageError struct {
	Timestamp    time.Time
	Operation    string
	FilePath     string
	ErrorMessage string
	RetryCount   int
}

var Globalsig_ss *int
var Global_sig_ss_Mutex *sync.Mutex

var Global_constant_config *utils.Ss_constant_config
var Global_screenshot_gap_Mutex *sync.Mutex
var Global_cache_path_Mutex *sync.Mutex
var Global_cache_path_instant_Mutex *sync.Mutex

var Global_database *sql.DB
var Global_database_managebot *sql.DB
var Global_database_net *sql.DB

var Global_logFile *os.File

var Global_safe_file_lock *utils.Safe_file_lock

var Global_map_image map[int]map[int64]*image.RGBA
var Global_map_image_Mutex *sync.Mutex
var Global_map_num_display map[int64]int
var Global_map_num_display_Mutex *sync.Mutex

// state identitier
var Global_screenshot_status int
var Global_screenshot_status_Mutex *sync.Mutex

// control the cache behavior
var Global_store int

// Storage error tracking
const MaxStorageErrors = 20

var Global_storage_errors []StorageError
var Global_storage_errors_mutex *sync.Mutex

// AddStorageError appends an error to the ring buffer with FIFO eviction
func AddStorageError(operation, filePath, errorMessage string, retryCount int) {
	Global_storage_errors_mutex.Lock()
	defer Global_storage_errors_mutex.Unlock()

	err := StorageError{
		Timestamp:    time.Now(),
		Operation:    operation,
		FilePath:     filePath,
		ErrorMessage: errorMessage,
		RetryCount:   retryCount,
	}

	// If buffer is full, remove oldest (FIFO)
	if len(Global_storage_errors) >= MaxStorageErrors {
		Global_storage_errors = Global_storage_errors[1:]
	}

	Global_storage_errors = append(Global_storage_errors, err)
}

// GetStorageErrors returns a formatted string of all errors for display
func GetStorageErrors() string {
	Global_storage_errors_mutex.Lock()
	defer Global_storage_errors_mutex.Unlock()

	if len(Global_storage_errors) == 0 {
		return "No storage errors recorded"
	}

	result := "Recent Storage Errors:"
	for _, err := range Global_storage_errors {
		result += "\n[" + err.Timestamp.Format("2006-01-02 15:04:05") +
			"] Op: " + err.Operation +
			" | Path: " + err.FilePath +
			" | Retries: " + string(rune('0'+err.RetryCount)) +
			" | Error: " + err.ErrorMessage
	}
	return result
}
