package library_manager

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"screenshot_server/Global"
	"screenshot_server/image_manipulation"
	"screenshot_server/utils"
	"strings"
	"sync"
	"sync/atomic"

	_ "github.com/mattn/go-sqlite3"
)

type library_parameter struct {
	timestamp string
	path      string
}

const defaultMachineID = "default"

// archiveBatchSize is the number of rows written per SQLite transaction when
// archiving screenshots. Committing once per batch instead of once per row
// avoids a disk sync for every screenshot.
const archiveBatchSize = 1000

// archiveWorkerCount bounds the goroutines used for metadata extraction and
// file moves, both of which are I/O bound.
var archiveWorkerCount = min(runtime.NumCPU(), 8)

// schemaReady is set once the screenshots schema has been ensured, so the
// migration/backfill statements do not rescan the table on every archive run.
var schemaReady atomic.Bool

func Init_database() *sql.DB {
	db, err := sql.Open("sqlite3", Global.Global_constant_config.Database_path)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func init_library_parameter() library_parameter {
	library_parameter := library_parameter{}
	library_parameter.path = Global.Global_constant_config.Cache_path
	return library_parameter
}

func hashStringSHA256(input string) string {
	hasher := sha256.New()
	hasher.Write([]byte(input))
	hashBytes := hasher.Sum(nil)
	return hex.EncodeToString(hashBytes)
}

func generateDefaultMachineScreenshotID(fileName string) string {
	return hashStringSHA256(defaultMachineID + ":" + fileName)
}

func ensureScreenshotsTable(db *sql.DB) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS screenshots (
		id TEXT PRIMARY KEY NOT NULL,
		hash TEXT NULL,
		hash_kind TEXT NULL,
		year INT NULL,
		month INT NULL,
		day INT NULL,
		hour INT NULL,
		minute INT NULL,
		second INT NULL,
		display_num INT NULL,
		file_name TEXT,
		machine_id TEXT DEFAULT 'default'
	);`
	_, err := db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	return nil
}

func EnsureScreenshotsMachineIDSchema(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}

	if err := ensureScreenshotsTable(db); err != nil {
		return err
	}

	_, err := db.Exec(`ALTER TABLE screenshots ADD COLUMN machine_id TEXT DEFAULT 'default'`)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
		return fmt.Errorf("failed to add machine_id column: %w", err)
	}

	if _, err := db.Exec(`UPDATE screenshots SET machine_id = 'default' WHERE machine_id IS NULL OR machine_id = ''`); err != nil {
		return fmt.Errorf("failed to backfill machine_id values: %w", err)
	}

	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_machine_display ON screenshots(machine_id, display_num)`); err != nil {
		return fmt.Errorf("failed to create idx_machine_display: %w", err)
	}

	// Dedup lookups match on (machine_id, file_name); without this index every
	// lookup is a full table scan.
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_machine_file_name ON screenshots(machine_id, file_name)`); err != nil {
		return fmt.Errorf("failed to create idx_machine_file_name: %w", err)
	}

	return nil
}

func create_database() error {
	if schemaReady.Load() {
		return nil
	}
	err := EnsureScreenshotsMachineIDSchema(Global.Global_database)
	if err != nil {
		// Capture error instead of crashing
		Global.AddStorageError("create_database", "", err.Error(), 0)
		return err
	}
	schemaReady.Store(true)
	fmt.Println("Table created successfully")
	return nil
}

// archiveRecord is a screenshot file with its metadata already extracted, ready
// to be written to the database.
type archiveRecord struct {
	fileName string
	id       string
	meta     image_manipulation.ImageMeta
	hasMeta  bool
}

func prepareArchiveRecord(file string) archiveRecord {
	fileName := filepath.Base(file)
	record := archiveRecord{
		fileName: fileName,
		id:       generateDefaultMachineScreenshotID(fileName),
	}
	meta, err := image_manipulation.Substract_Meta_from_file(file)
	if err == nil {
		record.meta = meta
		record.hasMeta = true
	}
	return record
}

// prepareArchiveRecords extracts metadata for files in parallel, preserving order.
func prepareArchiveRecords(file_list []string, numWorkers int) []archiveRecord {
	records := make([]archiveRecord, len(file_list))
	if numWorkers < 1 {
		numWorkers = 1
	}

	var wg sync.WaitGroup
	indexes := make(chan int, len(file_list))
	for i := range file_list {
		indexes <- i
	}
	close(indexes)

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range indexes {
				records[i] = prepareArchiveRecord(file_list[i])
			}
		}()
	}
	wg.Wait()
	return records
}

type archiveStatements struct {
	delete     *sql.Stmt
	insert     *sql.Stmt
	insertNull *sql.Stmt
}

func prepareArchiveStatements(tx *sql.Tx) (*archiveStatements, error) {
	stmts := &archiveStatements{}
	var err error
	if stmts.delete, err = tx.Prepare(`DELETE FROM screenshots WHERE id = ? OR (file_name = ? AND machine_id = ?)`); err != nil {
		return nil, err
	}
	if stmts.insert, err = tx.Prepare(`INSERT INTO screenshots (id, hash, hash_kind, year, month, day, hour, minute, second, display_num, file_name, machine_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`); err != nil {
		stmts.close()
		return nil, err
	}
	if stmts.insertNull, err = tx.Prepare(`INSERT INTO screenshots (id, file_name, machine_id) VALUES (?, ?, ?)`); err != nil {
		stmts.close()
		return nil, err
	}
	return stmts, nil
}

func (s *archiveStatements) close() {
	for _, stmt := range []*sql.Stmt{s.delete, s.insert, s.insertNull} {
		if stmt != nil {
			stmt.Close()
		}
	}
}

// write overwrites any existing row for the record's id or file name.
func (s *archiveStatements) write(record archiveRecord) error {
	if _, err := s.delete.Exec(record.id, record.fileName, defaultMachineID); err != nil {
		return fmt.Errorf("failed to delete existing entry: %w", err)
	}
	if !record.hasMeta {
		_, err := s.insertNull.Exec(record.id, record.fileName, defaultMachineID)
		return err
	}
	meta := record.meta
	_, err := s.insert.Exec(record.id, fmt.Sprintf("%d", meta.Hash), meta.HashKind, meta.Year, meta.Month, meta.Day, meta.Hour, meta.Minute, meta.Second, meta.DisplayNum, record.fileName, defaultMachineID)
	return err
}

// insert_batch_database writes all records in a single transaction.
func insert_batch_database(records []archiveRecord, database *sql.DB) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	stmts, err := prepareArchiveStatements(tx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmts.close()

	for _, record := range records {
		if err := stmts.write(record); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("batch insert failed on %s: %w", record.fileName, err)
		}
	}
	return tx.Commit()
}

// insert_archive_records writes records as one transaction, falling back to
// one transaction per record so a single bad row cannot block the rest.
func insert_archive_records(records []archiveRecord, database *sql.DB) {
	if len(records) == 0 {
		return
	}
	err := insert_batch_database(records, database)
	if err == nil {
		return
	}
	fmt.Printf("Batch insert of %d records failed, falling back to per-record inserts: %v\n", len(records), err)
	Global.AddStorageError("insert_batch_database", "", err.Error(), 0)

	single_task_insert_record := func(args ...interface{}) error {
		return insert_batch_database([]archiveRecord{args[0].(archiveRecord)}, database)
	}
	for _, record := range records {
		utils.Retry_single_task(single_task_insert_record, Global.Globalsig_ss, record)
	}
}

func insert_data_database_batched(file_list []string, batchSize int, database *sql.DB) {
	for start := 0; start < len(file_list); start += batchSize {
		end := min(start+batchSize, len(file_list))
		records := prepareArchiveRecords(file_list[start:end], archiveWorkerCount)
		insert_archive_records(records, database)
		fmt.Printf("archived %d/%d records to database\n", end, len(file_list))
		if *Global.Globalsig_ss == 0 {
			return
		}
	}
}

func remove_cache_to_memimg(file string) error {
	img_path := Global.Global_constant_config.Img_path

	// Ensure destination directory exists
	err := utils.EnsureDirectoryExists(img_path)
	if err != nil {
		Global.AddStorageError("remove_cache_to_memimg", file, "failed to create img_path directory: "+err.Error(), 0)
		return fmt.Errorf("failed to create img_path directory %s: %w", img_path, err)
	}

	fileName := filepath.Base(file)
	newPath := filepath.Join(img_path, fileName)
	err = utils.Move_file(file, newPath)
	if err != nil {
		// Capture error instead of crashing - file remains in cache
		Global.AddStorageError("remove_cache_to_memimg", file, err.Error(), 0)
		return fmt.Errorf("failed to move file %s to %s: %w", file, newPath, err)
	}
	return nil
}

// remove_cache_to_memimg_manager moves files in parallel and returns the files
// that could not be moved, in input order.
func remove_cache_to_memimg_manager(file_list []string, numWorkers int) []string {
	failed := make([]bool, len(file_list))
	if numWorkers < 1 {
		numWorkers = 1
	}

	var wg sync.WaitGroup
	indexes := make(chan int, len(file_list))
	for i := range file_list {
		indexes <- i
	}
	close(indexes)

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range indexes {
				if err := remove_cache_to_memimg(file_list[i]); err != nil {
					Global.AddStorageError("Insert_library", file_list[i], "move failed: "+err.Error(), 0)
					failed[i] = true
				}
			}
		}()
	}
	wg.Wait()

	failedMoves := []string{}
	for i, file := range file_list {
		if failed[i] {
			failedMoves = append(failedMoves, file)
		}
	}
	return failedMoves
}

func Insert_library(file_list []string) error {
	// library_parameter := init_library_parameter()
	// cache_path := Global_constant_config.cache_path
	// file_list := get_target_file_path(cache_path)
	single_task_create_database := func(args ...interface{}) error {
		return create_database()
	}
	utils.Retry_single_task(single_task_create_database, Global.Globalsig_ss)

	insert_data_database_batched(file_list, archiveBatchSize, Global.Global_database)

	// Track failed moves so files stay in cache
	failedMoves := remove_cache_to_memimg_manager(file_list, archiveWorkerCount)
	if len(failedMoves) > 0 {
		return fmt.Errorf("failed to move %d files (kept in cache): %v", len(failedMoves), failedMoves)
	}
	return nil
}

// filter_missing_files returns the files that have no row in the database,
// checking them all inside one read transaction.
func filter_missing_files(file_list []string, database *sql.DB) ([]string, error) {
	tx, err := database.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`SELECT EXISTS(SELECT 1 FROM screenshots WHERE id = ? OR (file_name = ? AND machine_id = ?))`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	missing := []string{}
	for _, file := range file_list {
		fileName := filepath.Base(file)
		var exists bool
		if err := stmt.QueryRow(generateDefaultMachineScreenshotID(fileName), fileName, defaultMachineID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("failed to query %s: %w", file, err)
		}
		if !exists {
			missing = append(missing, file)
		}
	}
	return missing, nil
}

// insert_missing_data_database_batched inserts rows for files not yet in the
// database, batchSize files at a time.
func insert_missing_data_database_batched(file_list []string, batchSize int, database *sql.DB) {
	task_filter_missing_files := func(args ...interface{}) (interface{}, error) {
		return filter_missing_files(args[0].([]string), database)
	}
	for start := 0; start < len(file_list); start += batchSize {
		end := min(start+batchSize, len(file_list))
		missing, _ := utils.Retry_task(task_filter_missing_files, Global.Globalsig_ss, file_list[start:end]).([]string)
		records := prepareArchiveRecords(missing, archiveWorkerCount)
		insert_archive_records(records, database)
		if *Global.Globalsig_ss == 0 {
			return
		}
	}
}

func Memimg_checking_robot() {
	img_path := Global.Global_constant_config.Img_path
	task_get_target_file_path_name := func(args ...interface{}) (interface{}, error) {
		input := args[0].(string)
		return utils.Get_target_file_path_name(input, "png")
	}
	get_target_file_path_name_return_img_path := utils.Retry_task(task_get_target_file_path_name, Global.Globalsig_ss, img_path).(utils.Get_target_file_path_name_return)
	file_path_list := get_target_file_path_name_return_img_path.Files

	single_task_create_database := func(args ...interface{}) error {
		return create_database()
	}
	utils.Retry_single_task(single_task_create_database, Global.Globalsig_ss)

	insert_missing_data_database_batched(file_path_list, archiveBatchSize, Global.Global_database_managebot)
	fmt.Println("memimg_checking_robot done round")
}

func Tidy_data_database() error {
	deleteSQL := `DELETE FROM screenshots WHERE file_name IS NULL`
	_, err := Global.Global_database.Exec(deleteSQL)
	if err != nil {
		fmt.Printf("Failed to delete: %v\n", err)
		return err
	}
	return nil
}
