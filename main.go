package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"screenshot_server/Global"
	"screenshot_server/image_manipulation"
	"screenshot_server/library_manager"
	"screenshot_server/utils"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
)

func init_Global_file_lock() error {
	var err error
	Global.Global_safe_file_lock.File_lock, err = utils.Get_target_file_name(Global.Global_constant_config.Cache_path, "png")
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
	Global.Global_logFile = logFile
	os.Stdout = Global.Global_logFile

	fmt.Println("Datetime: " + utils.GetDatetime())
	fmt.Println("Begin recording")
}

func closeLog() {
	fmt.Println("Datetime: " + utils.GetDatetime())
	fmt.Println("End recording")
	Global.Global_logFile.Close()
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
			distance := image_manipulation.Img_distance(img_brfore, img)
			if distance < 3 {
				map_image[i] = img
				continue
			} else {
				map_image[i] = img
			}
		} else {
			map_image[i] = img
		}
		ahash, _ := image_manipulation.AverageHash(img)
		fileName := fmt.Sprintf("%s_%d_%dx%d_%d.png", currentTime, i, bounds.Dx(), bounds.Dy(), ahash.Hash)
		filePath := fmt.Sprintf("./cache/%s", fileName)
		task_os_create := func(args ...interface{}) (interface{}, error) {
			file, err := os.Create(args[0].(string))
			return file, err
		}
		file := utils.Retry_task(task_os_create, Global.Globalsig_ss, filePath).(*os.File)
		defer file.Close()
		png.Encode(file, img)

		go func() {
			image_manipulation.Wirte_Meta_to_file(filePath, fileName, img)
			Global.Global_safe_file_lock.Lock.Lock()
			Global.Global_safe_file_lock.File_lock = append(Global.Global_safe_file_lock.File_lock, fileName)
			Global.Global_safe_file_lock.Lock.Unlock()
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
		if *Global.Globalsig_ss == 1 {
			continue
		}
		if *Global.Globalsig_ss == 0 {
			break
		}
		if *Global.Globalsig_ss == 2 {
			for {
				if *Global.Globalsig_ss == 1 {
					break
				} else if *Global.Globalsig_ss == 2 {
					continue
				} else if *Global.Globalsig_ss == 0 {
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
		file_num := utils.Retry_task(task_get_target_file_num, Global.Globalsig_ss, Global.Global_constant_config.Cache_path).(int)
		if file_num > 50 {
			cache_path := Global.Global_constant_config.Cache_path
			get_target_file_path_name_return := utils.Retry_task(task_get_target_file_path_name, Global.Globalsig_ss, cache_path).(utils.Get_target_file_path_name_return)
			file_path_list := get_target_file_path_name_return.Files
			file_name_list := get_target_file_path_name_return.FileNames
			for {
				time.Sleep(5 * time.Second)
				unlocked := library_manager.Check_if_locked(file_name_list)
				fmt.Println("unlocked : ", unlocked)
				if unlocked {
					library_manager.Remove_lock(file_name_list)
					library_manager.Insert_library(file_path_list)
					break
				} else {
					continue
				}
			}
		}
		if *Global.Globalsig_ss == 0 {
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
				library_manager.Memimg_checking_robot()
			}()

		case <-status_Ticker.C:
			if *Global.Globalsig_ss == 0 {
				break loop
			}

		default:
			time.Sleep(1 * time.Second)
		}
	}
}

func thread_tidy_data_database() {
	single_task_tidy_data_database := func(args ...interface{}) error {
		return library_manager.Tidy_data_database()
	}
	for {
		time.Sleep(5 * time.Second)
		utils.Retry_single_task(single_task_tidy_data_database, Global.Globalsig_ss)
		if *Global.Globalsig_ss == 0 {
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
	Global.Global_constant_config.Init_ss_constant_config()
	fmt.Println(Global.Global_constant_config.Screenshot_second)

	path_cache := Global.Global_constant_config.Cache_path
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

	Global.Global_safe_file_lock = new(utils.Safe_file_lock)
	Global.Global_safe_file_lock.Lock = new(sync.Mutex)

	initControlFile()
	init_Global_file_lock()
	Global.Globalsig_ss = new(int)
	*Global.Globalsig_ss = 1
	Global.Global_sig_ss_Mutex = new(sync.Mutex)

	Global.Global_database = library_manager.Init_database()
	Global.Global_database_net = library_manager.Init_database()
}

func close_program() {
	closeLog()
	Global.Global_database.Close()
	Global.Global_database_net.Close()
	time.Sleep(5 * time.Second) // make sure all zombie goroutine get the stop signal before Globalss_sig is released!
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
