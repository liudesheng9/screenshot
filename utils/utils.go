package utils

import (
	"fmt"
	"image"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Task func(args ...interface{}) (interface{}, error)
type Single_Task func(args ...interface{}) error

// retry method
func Retry_task(task Task, sig_ss *int, args ...interface{}) interface{} {
	for {
		result, err := task(args...)
		if err == nil {
			return result
		} else {
			fmt.Printf("Error: %v\n", err)
			time.Sleep(5 * time.Second)
			if *sig_ss == 0 {
				return result
			}
		}
	}
}

func Retry_single_task(task Single_Task, sig_ss *int, args ...interface{}) {
	for {
		err := task(args...)
		if err == nil {
			return
		} else {
			fmt.Printf("Error: %v\n", err)
			time.Sleep(5 * time.Second)
			if *sig_ss == 0 {
				return
			}
		}
	}
}

type Safe_connection struct {
	Conn net.Conn
	Lock *sync.Mutex
}

type Get_target_file_path_name_return struct {
	Files     []string
	FileNames []string
}

func Get_target_file_path(root string, suffix string) []string {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(path, "."+suffix) {
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

func Get_target_file_path_name(root string, suffix string) (Get_target_file_path_name_return, error) {
	var files []string
	var fileNames []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(path, "."+suffix) {
			return nil
		}
		files = append(files, path)
		fileName := filepath.Base(path)
		fileNames = append(fileNames, fileName)
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to walk path: %v", err)
		return_data := Get_target_file_path_name_return{nil, nil}
		return return_data, err
	}
	return_data := Get_target_file_path_name_return{files, fileNames}
	return return_data, nil
}
func Get_target_file_name(root string, suffix string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(path, "."+suffix) {
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

func Get_target_file_num(root string, suffix string) (int, error) {
	var i int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(path, "."+suffix) {
			return nil
		}
		i++
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to walk path: %v", err)
		return 0, err
	}
	return i, nil
}

type Ss_constant_config struct {
	Cache_path        string
	Img_path          string
	Dump_path         string
	Database_path     string
	Toml_path         string
	Screenshot_second int
	Tcp_port          int
}

func (c *Ss_constant_config) Init_ss_constant_config() {
	c.Cache_path = "./cache"
	c.Img_path = "./img"
	c.Dump_path = "./dump"
	c.Database_path = "./example.db"
	c.Toml_path = "./config.toml"
	c.Screenshot_second = 2
}

type Date struct {
	Year  int
	Month int
	Day   int
}

func GetDatetime() string {
	currentTime := time.Now()
	currentTimeStr := currentTime.String()

	currentTimeStr = strings.Replace(currentTimeStr, ":", "_", -1)
	currentTimeStr = strings.Replace(currentTimeStr, "-", "_", -1)
	currentTimeStr = strings.Replace(currentTimeStr, " ", "*", -1)
	currentTimeStr = strings.Replace(currentTimeStr, "_", "", -1)
	currentTimeStr = strings.Replace(currentTimeStr, "*", "_", -1)
	currentTimeStr = currentTimeStr[:15]

	return currentTimeStr
}

func Decode_dateTimeStr(dateTimeStr string, sig_ss *int) Date {
	task_strconv_atoi := func(args ...interface{}) (interface{}, error) {
		return strconv.Atoi(args[0].(string))
	}
	year := Retry_task(task_strconv_atoi, sig_ss, dateTimeStr[:4]).(int)
	month := Retry_task(task_strconv_atoi, sig_ss, dateTimeStr[4:6]).(int)
	day := Retry_task(task_strconv_atoi, sig_ss, dateTimeStr[6:8]).(int)

	return Date{year, month, day}

}

type Safe_file_lock struct {
	Lock      *sync.Mutex
	File_lock []string
}

func In_string_list(query string, list []string) bool {
	for _, item := range list {
		if query == item {
			return true
		}
	}
	return false
}

func In_string_list_index(query string, list []string) int {
	for index, item := range list {
		if query == item {
			return index
		}
	}
	return -1
}

type Image_thread_id struct {
	Img *image.RGBA
	Id  int64
}
