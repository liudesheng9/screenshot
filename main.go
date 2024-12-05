package main

import (
	"database/sql"
	"fmt"
	"image"
	"image/png"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
)

var Globalsig_ss int
var Global_constant_config ss_constant_config
var Global_database *sql.DB
var Global_database_net *sql.DB
var Global_sig_ss_Mutex sync.Mutex
var Global_logFile *os.File
var Global_file_lock_Mutex sync.Mutex
var Global_file_lock []string

type Task func(args ...interface{}) (interface{}, error)
type single_Task func(args ...interface{}) error

// retry method
func retry_task(task Task, args ...interface{}) interface{} {
	for {
		result, err := task(args...)
		if err == nil {
			return result
		} else {
			fmt.Printf("Error: %v\n", err)
			time.Sleep(5 * time.Second)
		}
	}
}

func retry_single_task(task single_Task, args ...interface{}) {
	for {
		err := task(args...)
		if err == nil {
			return
		} else {
			fmt.Printf("Error: %v\n", err)
			time.Sleep(5 * time.Second)
		}
	}
}

type date struct {
	year  int
	month int
	day   int
}

func init_Global_file_lock() error {
	var err error
	Global_file_lock, err = get_target_file_name(Global_constant_config.cache_path)
	if err != nil {
		return err
	}
	return nil
}

func initLog() {
	logFile, err := os.OpenFile("./log.txt", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("Cannot open log file: %v\n", err)
		return
	}
	Global_logFile = logFile
	os.Stdout = Global_logFile

	fmt.Println("Datetime: " + getDatetime())
	fmt.Println("Begin recording")
}

func closeLog() {
	fmt.Println("Datetime: " + getDatetime())
	fmt.Println("End recording")
	Global_logFile.Close()
}

func initControlFile() {
	file, err := os.Create("control.txt")
	if err != nil {
		fmt.Println(err)
	}
	file.WriteString("1")
	defer file.Close()
}

func getDatetime() string {
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

func decode_dateTimeStr(dateTimeStr string) date {
	task_strconv_atoi := func(args ...interface{}) (interface{}, error) {
		return strconv.Atoi(args[0].(string))
	}
	year := retry_task(task_strconv_atoi, dateTimeStr[:4]).(int)
	month := retry_task(task_strconv_atoi, dateTimeStr[4:6]).(int)
	day := retry_task(task_strconv_atoi, dateTimeStr[6:8]).(int)

	return date{year, month, day}

}

func screenshotExec(map_image map[int]*image.RGBA) {
	n := screenshot.NumActiveDisplays()
	currentTime := getDatetime()
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)

		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			fmt.Printf("CaptureRect failed: %v\n", err)
			continue
		}

		img_brfore := map_image[i]
		if img_brfore != nil {
			distance := img_distance(img_brfore, img)
			if distance < 3 {
				map_image[i] = img
				continue
			} else {
				map_image[i] = img
			}
		} else {
			map_image[i] = img
		}
		ahash, _ := AverageHash(img)
		fileName := fmt.Sprintf("%s_%d_%dx%d_%d.png", currentTime, i, bounds.Dx(), bounds.Dy(), ahash.hash)
		filePath := fmt.Sprintf("./cache/%s", fileName)
		task_os_create := func(args ...interface{}) (interface{}, error) {
			file, err := os.Create(args[0].(string))
			return file, err
		}
		file := retry_task(task_os_create, filePath).(*os.File)
		defer file.Close()
		png.Encode(file, img)

		go func() {
			wirte_Meta_to_file(filePath, fileName, img)
			Global_file_lock_Mutex.Lock()
			Global_file_lock = append(Global_file_lock, fileName)
			Global_file_lock_Mutex.Unlock()
		}()
		fmt.Printf("#%d : %v \"%s\"\n", i, bounds, fileName)
	}
}

func threadScreenshot() {
	map_image := make(map[int]*image.RGBA)
	for {
		go func() {
			screenshotExec(map_image)
		}()
		time.Sleep(2 * time.Second)
		if Globalsig_ss == 1 {
			continue
		}
		if Globalsig_ss == 0 {
			break
		}
		if Globalsig_ss == 2 {
			for {
				if Globalsig_ss == 1 {
					break
				} else if Globalsig_ss == 2 {
					continue
				} else if Globalsig_ss == 0 {
					break
				}
				time.Sleep(5 * time.Second)
			}
		}
	}
}

/*
	func threadControl() {
		for {
			time.Sleep(5 * time.Second)
			// open control.txt
			file, err := os.Open("control.txt")
			if err != nil {
				fmt.Printf("Control thread fatal error: %v\n", err)
				fmt.Println("Please end the program manually.")
			}
			// read control.txt
			buf := make([]byte, 100)
			n, err := file.Read(buf)
			if err != nil {
				fmt.Printf("Control thread fatal error: %v\n", err)
				fmt.Println("Please end the program manually.")
			}
			// close control.txt
			file.Close()
			control := string(buf[:n])
			if control == "0" {
				Global_sig_ss_Mutex.Lock()
				Globalsig_ss = 0
				Global_sig_ss_Mutex.Unlock()
				break
			}
			if control == "2" {
				Global_sig_ss_Mutex.Lock()
				Globalsig_ss = 2
				Global_sig_ss_Mutex.Unlock()
				continue
			}
			if control == "1" {
				Global_sig_ss_Mutex.Lock()
				Globalsig_ss = 1
				Global_sig_ss_Mutex.Unlock()
				continue
			}
		}
	}
*/
func thread_manage_library() {
	task_get_target_file_num := func(args ...interface{}) (interface{}, error) {
		input := args[0].(string)
		return get_target_file_num(input)
	}
	task_get_target_file_path_name := func(args ...interface{}) (interface{}, error) {
		input := args[0].(string)
		return get_target_file_path_name(input)
	}
	for {
		time.Sleep(5 * time.Second)
		file_num := retry_task(task_get_target_file_num, Global_constant_config.cache_path).(int)
		if file_num > 50 {
			cache_path := Global_constant_config.cache_path
			get_target_file_path_name_return := retry_task(task_get_target_file_path_name, cache_path).(get_target_file_path_name_return)
			file_path_list := get_target_file_path_name_return.files
			file_name_list := get_target_file_path_name_return.fileNames
			for {
				time.Sleep(5 * time.Second)
				unlocked := check_if_locked(file_name_list)
				fmt.Println("unlocked : ", unlocked)
				if unlocked {
					remove_lock(file_name_list)
					insert_library(file_path_list)
					break
				} else {
					continue
				}
			}
		}
		if Globalsig_ss == 0 {
			break
		}
	}
}

func thread_memimg_checking() {
	mem_check_Ticker := time.NewTicker(20 * time.Minute)
	status_Ticker := time.NewTicker(5 * time.Second)
loop:
	for {
		select {
		case <-mem_check_Ticker.C:
			go func() {
				memimg_checking_robot()
			}()

		case <-status_Ticker.C:
			if Globalsig_ss == 0 {
				break loop
			}

		default:
			time.Sleep(1 * time.Second)
		}
	}
}

func thread_tidy_data_database() {
	single_task_tidy_data_database := func(args ...interface{}) error {
		return tidy_data_database()
	}
	for {
		time.Sleep(5 * time.Second)
		retry_single_task(single_task_tidy_data_database)
		if Globalsig_ss == 0 {
			break
		}
	}
}

func thread_tcp_communication() {
	control_process_tcp()
}

func init_program() {
	// autostartInit()
	initLog()
	Global_constant_config.init_ss_constant_config()
	path_cache := Global_constant_config.cache_path
	err := os.MkdirAll(path_cache, os.ModePerm)
	initControlFile()
	init_Global_file_lock()

	if err != nil {
		fmt.Println(err)
	}
	Globalsig_ss = 1
	Global_database = init_database()
	Global_database_net = init_database()
}

func close_program() {
	closeLog()
	Global_database.Close()
	Global_database_net.Close()
}

func main() {
	// autostartInit()
	init_program()
	// gui_window := startGUI()

	var wg sync.WaitGroup
	wg.Add(5)
	go func() {
		threadScreenshot()
		wg.Done()
	}()
	/*
		go func() {
			threadControl()
			wg.Done()
		}()
	*/
	go func() {
		thread_manage_library()
		wg.Done()
	}()
	go func() {
		thread_memimg_checking()
		wg.Done()
	}()
	go func() {
		thread_tidy_data_database()
		wg.Done()
	}()
	go func() {
		thread_tcp_communication()
		wg.Done()
	}()
	wg.Wait()
	close_program()
}
