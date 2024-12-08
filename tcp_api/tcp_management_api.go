package tcp_api

import (
	"os"
	"screenshot_server/Global"
	"screenshot_server/utils"
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
	safe_conn.Lock.Lock()
	safe_conn.Conn.Write([]byte("invalid man command"))
	safe_conn.Lock.Unlock()
}
