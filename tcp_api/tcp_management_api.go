package tcp_api

import (
	"os"
	"screenshot_server/Global"
	"screenshot_server/init_config"
	"screenshot_server/library_manager"
	"screenshot_server/utils"
	"strconv"
	"strings"
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
	if len(recv_list) > 1 && recv_list[1] == "config" {
		execute_config_operation(safe_conn, recv_list[2:])
		return
	}
	safe_conn.Lock.Lock()
	safe_conn.Conn.Write([]byte("invalid man command"))
	safe_conn.Lock.Unlock()
}
