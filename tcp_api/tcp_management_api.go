package tcp_api

import (
	"fmt"
	"os"
	"screenshot_server/Global"
	"screenshot_server/init_config"
	"screenshot_server/library_manager"
	"screenshot_server/utils"
	"strconv"
	"strings"
	"sync"
	"time"
)

func dump_clean() {
	dump_root_path := Global.Global_constant_config.Dump_path
	task_get_target_file_path_name := func(args ...interface{}) (interface{}, error) {
		input := args[0].(string)
		return utils.Get_target_file_path_name(input, "txt")
	}
	single_task_os_remove := func(args ...interface{}) error {
		return os.Remove(args[0].(string))
	}
	get_target_file_path_name_return := utils.Retry_task(task_get_target_file_path_name, Global.Globalsig_ss, dump_root_path).(utils.Get_target_file_path_name_return)
	file_path_list := get_target_file_path_name_return.Files
	for _, file_path := range file_path_list {
		utils.Retry_single_task(single_task_os_remove, Global.Globalsig_ss, file_path)
	}
}

func execute_config_operation(safe_conn utils.Safe_connection, recv_list []string) {
	if len(recv_list) == 0 {
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("invalid config command"))
		safe_conn.Lock.Unlock()
		return
	}
	if len(recv_list) == 2 && recv_list[0] == "load" {
		// TODO: dynamic logic
		New_constant_config := init_config.Init_ss_constant_config_from_toml(recv_list[1])
		Old_constant_config := *Global.Global_constant_config
		if Old_constant_config.Screenshot_second != New_constant_config.Screenshot_second {
			Global.Global_constant_config.Screenshot_second = New_constant_config.Screenshot_second
		}
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("config loaded"))
		safe_conn.Lock.Unlock()
		return
	}
	if len(recv_list) == 2 && recv_list[0] == "screenshot_gap" {
		New_gap_second, err := strconv.Atoi(recv_list[1])
		if err != nil {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("invalid screenshot_gap value"))
			safe_conn.Lock.Unlock()
			return
		}
		Global.Global_screenshot_gap_Mutex.Lock()
		Global.Global_constant_config.Screenshot_second = New_gap_second
		Global.Global_screenshot_gap_Mutex.Unlock()
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("screen shot gap changed, new gap: " + strconv.Itoa(Global.Global_constant_config.Screenshot_second)))
		safe_conn.Lock.Unlock()
		return
	}
	if len(recv_list) == 2 && recv_list[0] == "cache_path" {
		//input check
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("checking input..."))
		safe_conn.Lock.Unlock()
		if recv_list[1][:2] != "./" {
			recv_list[1] = "./" + recv_list[1]
		}
		err := os.MkdirAll(recv_list[1], os.ModePerm)
		if err != nil {
			fmt.Println("make path failed: ", err)
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("make path failed"))
			safe_conn.Lock.Unlock()
			return
		}
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("make path success"))
		safe_conn.Lock.Unlock()

		Old_cache_path := Global.Global_constant_config.Cache_path
		if Old_cache_path == recv_list[1] {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("cache path not changed"))
			safe_conn.Lock.Unlock()
			return
		}

		Global.Global_cache_path_Mutex.Lock()
		Global.Global_cache_path_instant_Mutex.Lock()
		Global.Global_constant_config.Cache_path = recv_list[1]
		Global.Global_cache_path_instant_Mutex.Unlock()
		Global.Global_cache_path_Mutex.Unlock()

		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("cache path changed, new path: " + Global.Global_constant_config.Cache_path))
		safe_conn.Lock.Unlock()

		task_get_target_file_path_name := func(args ...interface{}) (interface{}, error) {
			input := args[0].(string)
			return utils.Get_target_file_path_name(input, "png")
		}
		task_get_target_file_num := func(args ...interface{}) (interface{}, error) {
			input := args[0].(string)
			return utils.Get_target_file_num(input, "png")
		}
		wg := sync.WaitGroup{}
		wg.Add(2)

		// remove imgs
		go func() {
			defer wg.Done()
			Global.Global_cache_path_Mutex.Lock()
			get_target_file_path_name_return := utils.Retry_task(task_get_target_file_path_name, Global.Globalsig_ss, Old_cache_path).(utils.Get_target_file_path_name_return)
			file_path_list := get_target_file_path_name_return.Files
			file_name_list := get_target_file_path_name_return.FileNames
			for {
				time.Sleep(5 * time.Second)
				unlocked := library_manager.Check_if_locked(file_name_list)
				fmt.Println("unlocked : ", unlocked)
				if unlocked {
					library_manager.Remove_lock(file_name_list)
					err := library_manager.Insert_library(file_path_list)
					if err != nil {
						fmt.Printf("Insert_library error during cache_path change: %v\n", err)
					}
					break
				} else {
					continue
				}

			}
			Global.Global_cache_path_Mutex.Unlock()

			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("move imgs done"))
			safe_conn.Lock.Unlock()
		}()

		// dump toml
		go func() {
			defer wg.Done()
			err = init_config.Encode_ss_constant_config_to_toml(*Global.Global_constant_config, "./config.toml")
			if err != nil {
				safe_conn.Lock.Lock()
				safe_conn.Conn.Write([]byte("dump toml failed"))
				safe_conn.Lock.Unlock()
				return
			}
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("dump toml success"))
			safe_conn.Lock.Unlock()
		}()

		wg.Wait()

		//remove old cache path
		go func() {

			for i := 0; i < 5; i++ {
				file_num := utils.Retry_task(task_get_target_file_num, Global.Globalsig_ss, Old_cache_path).(int)
				if file_num == 0 {
					err := os.RemoveAll(Old_cache_path)
					if err != nil {
						safe_conn.Lock.Lock()
						safe_conn.Conn.Write([]byte("remove old cache path failed, please remove it manually"))
						safe_conn.Conn.Write([]byte("error: " + err.Error()))
						safe_conn.Lock.Unlock()
						return
					}
					safe_conn.Lock.Lock()
					safe_conn.Conn.Write([]byte("remove old cache path success"))
					safe_conn.Lock.Unlock()
					return
				} else {
					time.Sleep(3 * time.Second)
					// fmt.Println("file num: ", file_num)
					continue
				}
			}
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("remove old cache path failed, please remove it manually"))
			safe_conn.Lock.Unlock()
		}()
		return
	}
	if len(recv_list) == 1 && recv_list[0] == "dump_toml" {
		err := init_config.Encode_ss_constant_config_to_toml(*Global.Global_constant_config, "./config.toml")
		if err != nil {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("dump toml failed"))
			safe_conn.Lock.Unlock()
			return
		}
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("dump toml success"))
		safe_conn.Lock.Unlock()
		return
	}

	safe_conn.Lock.Lock()
	safe_conn.Conn.Write([]byte("invalid config command"))
	safe_conn.Lock.Unlock()
}

func Execute_manager(safe_conn utils.Safe_connection, recv string) {
	recv_list := strings.Split(recv, " ")
	if len(recv_list) == 1 {
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("invalid man command"))
		safe_conn.Lock.Unlock()
		return
	}
	if len(recv_list) == 3 && recv_list[1] == "dump" && recv_list[2] == "clean" {
		dump_clean()
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("dump cleaned"))
		safe_conn.Lock.Unlock()
		return
	}
	if len(recv_list) == 3 && recv_list[1] == "mem" && recv_list[2] == "check" {
		go library_manager.Memimg_checking_robot()
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("Memory image checking robot started"))
		safe_conn.Lock.Unlock()
		return
	}
	if len(recv_list) == 3 && recv_list[1] == "tidy" && recv_list[2] == "database" {
		library_manager.Tidy_data_database()
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("Database tidied"))
		safe_conn.Lock.Unlock()
		return
	}
	if len(recv_list) == 2 && recv_list[1] == "status" {
		Global.Global_screenshot_status_Mutex.Lock()
		screenshot_status := Global.Global_screenshot_status
		Global.Global_screenshot_status_Mutex.Unlock()
		safe_conn.Lock.Lock()
		if screenshot_status <= 0 {
			safe_conn.Conn.Write([]byte("screenshot state: off"))
		} else {
			write := "\nscreenshot state: running"
			write += "\nrunning thread num: " + strconv.Itoa(screenshot_status)
			safe_conn.Conn.Write([]byte(write))
		}
		if Global.Global_store == 0 {
			safe_conn.Conn.Write([]byte("\nstore: off"))
		} else {
			safe_conn.Conn.Write([]byte("\nstore: on"))
		}
		safe_conn.Lock.Unlock()
		return
	}
	if len(recv_list) == 2 && recv_list[1] == "store" {
		Global.Global_store = 1
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("store on"))
		safe_conn.Lock.Unlock()
		return
	}
	if len(recv_list) == 2 && recv_list[1] == "nostore" {
		Global.Global_store = 0
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("store off"))
		safe_conn.Lock.Unlock()
		return
	}
	if len(recv_list) > 1 && recv_list[1] == "config" {
		execute_config_operation(safe_conn, recv_list[2:])
		return
	}
	if len(recv_list) == 3 && recv_list[1] == "store" && recv_list[2] == "errors" {
		execute_store_errors(safe_conn)
		return
	}
	safe_conn.Lock.Lock()
	safe_conn.Conn.Write([]byte("invalid man command"))
	safe_conn.Lock.Unlock()
}

func execute_store_errors(safe_conn utils.Safe_connection) {
	errorsText := Global.GetStorageErrors()
	safe_conn.Lock.Lock()
	safe_conn.Conn.Write([]byte(errorsText))
	safe_conn.Lock.Unlock()
}
