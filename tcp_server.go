package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

var Global_conn_list []*safe_connection
var Global_conn_list_lock sync.Mutex

type safe_connection struct {
	conn net.Conn
	lock *sync.Mutex
}

func dump_clean() {
	dump_root_path := Global_constant_config.dump_path
	task_get_target_file_path_name := func(args ...interface{}) (interface{}, error) {
		input := args[0].(string)
		return get_target_file_path_name(input, "txt")
	}
	single_task_os_remove := func(args ...interface{}) error {
		return os.Remove(args[0].(string))
	}
	get_target_file_path_name_return := retry_task(task_get_target_file_path_name, dump_root_path).(get_target_file_path_name_return)
	file_path_list := get_target_file_path_name_return.files
	for _, file_path := range file_path_list {
		retry_single_task(single_task_os_remove, file_path)
	}
}

func excute_recv_command(safe_conn safe_connection, recv string) {
	// delete start space and end space
	recv = strings.TrimSpace(recv)
	if recv == "0" {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 0
		Global_sig_ss_Mutex.Unlock()
		safe_conn.lock.Lock()
		safe_conn.conn.Write([]byte("set stop"))
		safe_conn.lock.Unlock()
		return
	}
	if recv == "1" {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 1
		Global_sig_ss_Mutex.Unlock()
		safe_conn.lock.Lock()
		safe_conn.conn.Write([]byte("set start"))
		safe_conn.lock.Unlock()
		return
	}
	if recv == "2" {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 2
		Global_sig_ss_Mutex.Unlock()
		safe_conn.lock.Lock()
		safe_conn.conn.Write([]byte("set pause"))
		safe_conn.lock.Unlock()
		return
	}
	if recv == "hello server" {
		safe_conn.lock.Lock()
		safe_conn.conn.Write([]byte("1"))
		safe_conn.lock.Unlock()
		return
	}
	if recv == "dump clean" {
		dump_clean()
		safe_conn.lock.Lock()
		safe_conn.conn.Write([]byte("dump cleaned"))
		safe_conn.lock.Unlock()
		return
	}
	if strings.Split(recv, " ")[0] == "sql" {
		execute_sql(safe_conn, recv)
		return
	}
	safe_conn.lock.Lock()
	safe_conn.conn.Write([]byte("received: " + recv))
	safe_conn.lock.Unlock()
}

func process_tcp(safe_conn safe_connection) {
	//关闭连接
	for {
		reader := bufio.NewReader(safe_conn.conn)
		var buf [128]byte
		n, err := reader.Read(buf[:])
		if err != nil {
			fmt.Printf("read from conn failed ,err:%v\n", err)
			break
		}
		recv := string(buf[:n])
		if recv == "exit" {
			fmt.Println("exit")
			break
		}
		go func() {
			excute_recv_command(safe_conn, recv)
		}()
	}

	Global_conn_list_lock.Lock()

	safe_conn.lock.Lock()
	safe_conn.conn.Write([]byte("server close"))
	_ = safe_conn.conn.Close()
	safe_conn.lock.Unlock()

	for i, v := range Global_conn_list {
		if v == &safe_conn {
			Global_conn_list = append(Global_conn_list[:i], Global_conn_list[i+1:]...)
			v.lock = nil
			//release safe_conn
			v = nil
			runtime.GC()
			break
		}
	}
	Global_conn_list_lock.Unlock()
}

func control_process_tcp() {
	//start service
	task_net_listen := func(args ...interface{}) (interface{}, error) {
		listen, err := net.Listen("tcp", args[0].(string))
		return listen, err
	}
	listen := retry_task(task_net_listen, "127.0.0.1:50021").(net.Listener)

	for {
		if Globalsig_ss == 0 {
			// close all connection
			Global_conn_list_lock.Lock()
			for _, v := range Global_conn_list {
				v.lock.Lock()
				v.conn.Write([]byte("server close"))
				_ = v.conn.Close()
				v.lock.Unlock()
				v.lock = nil
				v = nil
				runtime.GC()
			}
			Global_conn_list_lock.Unlock()
			break
		}
		//wait client
		listen.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second))
		conn, err := listen.Accept()
		if err != nil {
			if opErr, ok := err.(net.Error); ok && opErr.Timeout() {
				continue
			}
			fmt.Println("Error accepting connection:", err)
			break
		}

		conn_lock := sync.Mutex{}
		if conn_lock.TryLock() {
			conn_lock.Unlock()
		}
		safe_conn := safe_connection{conn: conn, lock: &conn_lock}
		Global_conn_list = append(Global_conn_list, &safe_conn)
		//start goroutine processs
		go process_tcp(safe_conn)
	}

}
