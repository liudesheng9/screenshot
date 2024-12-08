package tcp_api

import (
	"os"
	"screenshot_server/utils"
	"strings"
)

func dump_clean() {
	dump_root_path := Global_constant_config.Dump_path
	task_get_target_file_path_name := func(args ...interface{}) (interface{}, error) {
		input := args[0].(string)
		return utils.Get_target_file_path_name(input, "txt")
	}
	single_task_os_remove := func(args ...interface{}) error {
		return os.Remove(args[0].(string))
	}
	get_target_file_path_name_return := utils.Retry_task(task_get_target_file_path_name, Global_sig_ss, dump_root_path).(utils.Get_target_file_path_name_return)
	file_path_list := get_target_file_path_name_return.Files
	for _, file_path := range file_path_list {
		utils.Retry_single_task(single_task_os_remove, Global_sig_ss, file_path)
	}
}

func Execute_manager(safe_conn utils.Safe_connection, recv string, config utils.Ss_constant_config) {
	init_ss_constant_config(config)
	recv_list := strings.Split(recv, " ")
	if recv_list[1] == "dump" && recv_list[2] == "clean" && len(recv_list) == 3 {
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
