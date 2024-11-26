package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type library_parameter struct {
	timestamp string
	path      string
}

type get_target_file_path_name_return struct {
	files     []string
	fileNames []string
}

func init_database() *sql.DB {
	db, err := sql.Open("sqlite3", Global_constant_config.database_path)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func check_if_locked(filename_list []string) bool {
	map_lock := make(map[string]bool)
	Global_file_lock_Mutex.Lock()
	for _, item := range Global_file_lock {
		map_lock[item] = true
	}
	Global_file_lock_Mutex.Unlock()
	for _, filename := range filename_list {
		if !map_lock[filename] {
			return false
		}
	}
	return true
}

func remove_lock(filename_list []string) {
	Global_file_lock_Mutex.Lock()
	for _, filename := range filename_list {
		for i, item := range Global_file_lock {
			if item == filename {
				Global_file_lock = append(Global_file_lock[:i], Global_file_lock[i+1:]...)
				break
			}
		}
	}
	Global_file_lock_Mutex.Unlock()
}

func get_target_file_path(root string) []string {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(path, ".png") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to walk path: %v", err)
	}
	return files
}

func get_target_file_path_name(root string) (get_target_file_path_name_return, error) {
	var files []string
	var fileNames []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(path, ".png") {
			return nil
		}
		files = append(files, path)
		fileName := filepath.Base(path)
		fileNames = append(fileNames, fileName)
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to walk path: %v", err)
		return_data := get_target_file_path_name_return{nil, nil}
		return return_data, err
	}
	return_data := get_target_file_path_name_return{files, fileNames}
	return return_data, nil
}
func get_target_file_name(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(path, ".png") {
			return nil
		}
		fileName := filepath.Base(path)
		files = append(files, fileName)
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to walk path: %v", err)
		return nil, err
	}
	return files, nil
}

func get_target_file_num(root string) (int, error) {
	var i int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		i++
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to walk path: %v", err)
		return 0, err
	}
	return i, nil
}

func init_library_parameter() library_parameter {
	library_parameter := library_parameter{}
	library_parameter.path = Global_constant_config.cache_path
	return library_parameter
}

func hashStringSHA256(input string) string {
	hasher := sha256.New()
	hasher.Write([]byte(input))
	hashBytes := hasher.Sum(nil)
	return hex.EncodeToString(hashBytes)
}

func manage_library(file_list []string) {
	// library_parameter := init_library_parameter()
	// cache_path := Global_constant_config.cache_path
	// file_list := get_target_file_path(cache_path)

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
		file_name TEXT 
	);`
	_, err := Global_database.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	fmt.Println("Table created successfully")
	insertSQL := `INSERT INTO screenshots (id, hash, hash_kind, year, month, day, hour, minute, second, display_num, file_name) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	insertSQL_NULL := `INSERT INTO screenshots (id, file_name) VALUES (?, ?)`
	for _, file := range file_list {
		Meta_data, err := substract_Meta_from_file(file)
		if err != nil {
			_, err = Global_database.Exec(insertSQL_NULL, hashStringSHA256(filepath.Base(file)), filepath.Base(file))
			if err != nil {
				log.Fatalf("Failed to insert: %v", err)
				// return err
			}
			continue
		}
		Meta_map := convert_Meta_to_interface_map(Meta_data)
		fileName := filepath.Base(file)
		Meta_map["file_name"] = fileName
		_, err = Global_database.Exec(insertSQL, hashStringSHA256(filepath.Base(file)), fmt.Sprintf("%d", Meta_map["hash"]), Meta_map["hash_kind"], Meta_map["year"], Meta_map["month"], Meta_map["day"], Meta_map["hour"], Meta_map["minute"], Meta_map["second"], Meta_map["display_num"], Meta_map["file_name"])
		if err != nil {
			log.Fatalf("Failed to insert: %v", err)
			// return err
		}
	}
	img_path := Global_constant_config.img_path
	// move files to img_path
	for _, file := range file_list {
		fileName := filepath.Base(file)
		newPath := filepath.Join(img_path, fileName)
		err := os.Rename(file, newPath)
		if err != nil {
			log.Fatalf("Failed to move file: %v", err)
			// return err
		}
	}
}
