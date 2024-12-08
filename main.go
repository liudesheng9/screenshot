package main

import (
	"database/sql"
	"fmt"
	"image"
	"image/png"
	"os"
	"screenshot_server/utils"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
)

var Globalsig_ss int
var Global_constant_config utils.Ss_constant_config
var Global_database *sql.DB
var Global_database_net *sql.DB
var Global_sig_ss_Mutex sync.Mutex
var Global_logFile *os.File
var Global_file_lock_Mutex sync.Mutex
var Global_file_lock []string

func init_Global_file_lock() error {
	var err error
	Global_file_lock, err = utils.Get_target_file_name(Global_constant_config.Cache_path, "png")
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

	fmt.Println("Datetime: " + utils.GetDatetime())
	fmt.Println("Begin recording")
}

func closeLog() {
	fmt.Println("Datetime: " + utils.GetDatetime())
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

func screenshotExec(map_image map[int]*image.RGBA) {
	n := screenshot.NumActiveDisplays()
	currentTime := utils.GetDatetime()
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
		file := utils.Retry_task(task_os_create, Globalsig_ss, filePath).(*os.File)
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
		// time_duration := time.Duration(Global_constant_config.screenshot_second) * time.Second
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
		return utils.Get_target_file_num(input)
	}
	task_get_target_file_path_name := func(args ...interface{}) (interface{}, error) {
		input := args[0].(string)
		return utils.Get_target_file_path_name(input, "png")
	}
	for {
		time.Sleep(5 * time.Second)
		file_num := utils.Retry_task(task_get_target_file_num, Globalsig_ss, Global_constant_config.Cache_path).(int)
		if file_num > 50 {
			cache_path := Global_constant_config.Cache_path
			get_target_file_path_name_return := utils.Retry_task(task_get_target_file_path_name, Globalsig_ss, cache_path).(utils.Get_target_file_path_name_return)
			file_path_list := get_target_file_path_name_return.Files
			file_name_list := get_target_file_path_name_return.FileNames
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
	mem_check_Ticker := time.NewTicker(1 * time.Hour)
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
		utils.Retry_single_task(single_task_tidy_data_database, Globalsig_ss)
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
	// Global_constant_config = init_ss_constant_config_from_toml()
	Global_constant_config.Init_ss_constant_config()
	fmt.Println(Global_constant_config.Screenshot_second)

	path_cache := Global_constant_config.Cache_path
	err := os.MkdirAll(path_cache, os.ModePerm)
	if err != nil {
		fmt.Println(err)
	}

	// path_dump := Global_constant_config.dump_path
	path_dump := "./dump"
	err = os.MkdirAll(path_dump, os.ModePerm)
	if err != nil {
		fmt.Println(err)
	}

	initControlFile()
	init_Global_file_lock()

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
