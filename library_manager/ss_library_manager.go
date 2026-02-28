package library_manager

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"path/filepath"
	"screenshot_server/Global"
	"screenshot_server/image_manipulation"
	"screenshot_server/utils"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

type library_parameter struct {
	timestamp string
	path      string
}

const defaultMachineID = "default"

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

	return nil
}

func create_database() error {
	err := EnsureScreenshotsMachineIDSchema(Global.Global_database)
	if err != nil {
		// Capture error instead of crashing
		Global.AddStorageError("create_database", "", err.Error(), 0)
		return err
	}
	fmt.Println("Table created successfully")
	return nil
}

func insert_data_database(file string, database *sql.DB) error {
	insertSQL := `INSERT INTO screenshots (id, hash, hash_kind, year, month, day, hour, minute, second, display_num, file_name, machine_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	insertSQL_NULL := `INSERT INTO screenshots (id, file_name, machine_id) VALUES (?, ?, ?)`

	// Check if file already exists in database
	fileName := filepath.Base(file)
	fileID := generateDefaultMachineScreenshotID(fileName)

	// Check if record exists
	checkSQL := `SELECT EXISTS(SELECT 1 FROM screenshots WHERE id = ? OR (file_name = ? AND machine_id = ?))`
	var exists bool
	err := database.QueryRow(checkSQL, fileID, fileName, defaultMachineID).Scan(&exists)
	if err != nil {
		fmt.Printf("Failed to check if record exists: %v, %s, %s\n", err, file, fileID)
		return err
	}

	// Delete previous entry only if it exists
	if exists {
		deleteSQL := `DELETE FROM screenshots WHERE id = ? OR (file_name = ? AND machine_id = ?)`
		_, err := database.Exec(deleteSQL, fileID, fileName, defaultMachineID)
		if err != nil {
			fmt.Printf("Failed to delete existing entry: %v, %s, %s\n", err, file, fileID)
			return err
		}
	}

	// Continue with the regular insert process
	Meta_data, err := image_manipulation.Substract_Meta_from_file(file)
	if err != nil {
		_, err = database.Exec(insertSQL_NULL, fileID, fileName, defaultMachineID)
		if err != nil {
			fmt.Printf("Failed to insert: %v, %s, %s\n", err, file, fileID)
			return err
		}
		return nil
	}
	Meta_map := image_manipulation.Convert_Meta_to_interface_map(Meta_data)
	Meta_map["file_name"] = fileName
	_, err = database.Exec(insertSQL, fileID, fmt.Sprintf("%d", Meta_map["hash"]), Meta_map["hash_kind"], Meta_map["year"], Meta_map["month"], Meta_map["day"], Meta_map["hour"], Meta_map["minute"], Meta_map["second"], Meta_map["display_num"], Meta_map["file_name"], defaultMachineID)
	if err != nil {
		fmt.Printf("Failed to insert: %v, %s, %s\n", err, file, fileID)
		return err
	}
	return nil
}

func insert_data_database_worker_manager(file_list []string, numWorkers int, database *sql.DB) {
	numTasks := len(file_list)

	single_task_insert_data_database := func(args ...interface{}) error {
		return insert_data_database(args[0].(string), database)
	}
	var wg sync.WaitGroup

	tasks := make(chan string, numTasks)

	worker := func(id int, in <-chan string, wg *sync.WaitGroup) {
		defer wg.Done()
		for file := range in {
			utils.Retry_single_task(single_task_insert_data_database, Global.Globalsig_ss, file)
		}
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(i, tasks, &wg)
	}
	for _, file := range file_list {
		tasks <- file
	}

	// close task channel
	close(tasks)
	wg.Wait()
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

func remove_cache_to_memimg_manager(file_list []string) {
	single_task_remove_cache_to_memimg := func(args ...interface{}) error {
		return remove_cache_to_memimg(args[0].(string))
	}
	for _, file := range file_list {
		utils.Retry_single_task(single_task_remove_cache_to_memimg, Global.Globalsig_ss, file)
	}
}

func Insert_library(file_list []string) error {
	// library_parameter := init_library_parameter()
	// cache_path := Global_constant_config.cache_path
	// file_list := get_target_file_path(cache_path)
	single_task_create_database := func(args ...interface{}) error {
		return create_database()
	}
	utils.Retry_single_task(single_task_create_database, Global.Globalsig_ss)

	insert_data_database_worker_manager(file_list, 1, Global.Global_database)

	// Track failed moves so files stay in cache
	failedMoves := []string{}
	for _, file := range file_list {
		err := remove_cache_to_memimg(file)
		if err != nil {
			Global.AddStorageError("Insert_library", file, "move failed: "+err.Error(), 0)
			failedMoves = append(failedMoves, file)
		}
	}

	if len(failedMoves) > 0 {
		return fmt.Errorf("failed to move %d files (kept in cache): %v", len(failedMoves), failedMoves)
	}
	return nil
}

func query_data_exists_database(file string) (bool, error) {
	filename := filepath.Base(file)
	query_file_name := "SELECT EXISTS(SELECT 1 FROM screenshots WHERE file_name = ? AND machine_id = ?)"
	query_hashSHA256 := "SELECT EXISTS(SELECT 1 FROM screenshots WHERE id = ?)"
	var exists_file_name bool
	var exists_hashSHA256 bool
	err := Global.Global_database_managebot.QueryRow(query_file_name, filename, defaultMachineID).Scan(&exists_file_name)
	if err != nil {
		log.Fatalf("Failed to query: %v", err)
		return false, err
	}
	err = Global.Global_database_managebot.QueryRow(query_hashSHA256, generateDefaultMachineScreenshotID(filename)).Scan(&exists_hashSHA256)
	if err != nil {
		log.Fatalf("Failed to query: %v", err)
		return false, err
	}
	exists := exists_file_name || exists_hashSHA256
	return exists, nil
}

func query_data_insert_database(file string) error {
	task_query_data_exists_database := func(args ...interface{}) (interface{}, error) {
		return query_data_exists_database(args[0].(string))
	}
	exists := utils.Retry_task(task_query_data_exists_database, Global.Globalsig_ss, file).(bool)
	if exists {
		return nil
	}
	err := insert_data_database(file, Global.Global_database_managebot)
	if err != nil {
		return err
	}
	return nil
}

func insert_data_database_worker_manager_with_exist_bool(file_list []string, numWorkers int) {
	numTasks := len(file_list)

	single_task_query_data_insert_database := func(args ...interface{}) error {
		return query_data_insert_database(args[0].(string))
	}
	var wg sync.WaitGroup

	tasks := make(chan string, numTasks)

	worker := func(id int, in <-chan string, wg *sync.WaitGroup) {
		defer wg.Done()
		for file := range in {
			utils.Retry_single_task(single_task_query_data_insert_database, Global.Globalsig_ss, file)
		}
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(i, tasks, &wg)
	}
	for _, file := range file_list {
		tasks <- file
	}

	// close task channel
	close(tasks)
	wg.Wait()
}

func Memimg_checking_robot() {
	img_path := Global.Global_constant_config.Img_path
	task_get_target_file_path_name := func(args ...interface{}) (interface{}, error) {
		input := args[0].(string)
		return utils.Get_target_file_path_name(input, "png")
	}
	get_target_file_path_name_return_img_path := utils.Retry_task(task_get_target_file_path_name, Global.Globalsig_ss, img_path).(utils.Get_target_file_path_name_return)
	file_path_list := get_target_file_path_name_return_img_path.Files

	insert_data_database_worker_manager_with_exist_bool(file_path_list, 10)
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
