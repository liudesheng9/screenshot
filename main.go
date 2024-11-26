package main

import (
	"database/sql"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
)

var Globalsig_ss int
var Global_constant_config ss_constant_config
var Global_database *sql.DB
var Global_sig_ss_Mutex sync.Mutex
var Global_logFile *os.File
var Global_file_lock_Mutex sync.Mutex
var Global_file_lock []string

func init_Global_file_lock() {
	Global_file_lock = get_target_file_name(Global_constant_config.cache_path)
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
		panic(err)
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
		file, err := os.Create(filePath)
		if err != nil {
			panic(err)
		}
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
		screenshotExec(map_image)
		time.Sleep(5 * time.Second)
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

func threadControl() {
	for {
		time.Sleep(5 * time.Second)
		// open control.txt
		file, err := os.Open("control.txt")
		if err != nil {
			panic(err)
		}
		// read control.txt
		buf := make([]byte, 100)
		n, err := file.Read(buf)
		if err != nil {
			panic(err)
		}
		// close control.txt
		file.Close()
		control := string(buf[:n])
		if control == "0" {
			Globalsig_ss = 0
			break
		}
		if control == "2" {
			Globalsig_ss = 2
			continue
		}
		if control == "1" {
			Globalsig_ss = 1
			continue
		}
	}
}

func thread_manage_library() {
	for {
		time.Sleep(5 * time.Second)
		file_num := get_target_file_num(Global_constant_config.cache_path)
		if file_num > 20 {
			cache_path := Global_constant_config.cache_path
			file_path_list, file_name_list := get_target_file_path_name(cache_path)
			for {
				time.Sleep(5 * time.Second)
				unlocked := check_if_locked(file_name_list)
				fmt.Println("unlocked : ", unlocked)
				if unlocked {
					remove_lock(file_name_list)
					manage_library(file_path_list)
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

func init_program() {
	// autostartInit()
	initLog()
	Global_constant_config.init_ss_constant_config()
	path_cache := Global_constant_config.cache_path
	err := os.MkdirAll(path_cache, os.ModePerm)
	initControlFile()
	init_Global_file_lock()

	if err != nil {
		panic(err)
	}
	Globalsig_ss = 0
	Global_database = init_database()
}

func close_program() {
	closeLog()
	Global_database.Close()
}

func main() {
	// autostartInit()
	init_program()
	// gui_window := startGUI()

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		threadScreenshot()
		wg.Done()
	}()
	go func() {
		threadControl()
		wg.Done()
	}()
	go func() {
		thread_manage_library()
		wg.Done()
	}()
	wg.Wait()
	close_program()
}
