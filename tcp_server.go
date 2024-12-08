package main

import (
	"bufio"
	"fmt"
	"net"
	"runtime"
	"screenshot_server/tcp_api"
	"screenshot_server/utils"
	"strings"
	"sync"
	"time"
)

var Global_conn_list []*utils.Safe_connection
var Global_conn_list_lock sync.Mutex

func excute_recv_command(safe_conn utils.Safe_connection, recv string) {
	// delete start space and end space
	recv = strings.TrimSpace(recv)
	if recv == "0" {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 0
		Global_sig_ss_Mutex.Unlock()
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("set stop"))
		safe_conn.Lock.Unlock()
		return
	}
	if recv == "1" {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 1
		Global_sig_ss_Mutex.Unlock()
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("set start"))
		safe_conn.Lock.Unlock()
		return
	}
	if recv == "2" {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 2
		Global_sig_ss_Mutex.Unlock()
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("set pause"))
		safe_conn.Lock.Unlock()
		return
	}
	if recv == "hello server" {
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("1"))
		safe_conn.Lock.Unlock()
		return
	}
	if strings.Split(recv, " ")[0] == "man" {
		tcp_api.Execute_manager(safe_conn, recv, Global_constant_config)
		return
	}
	if strings.Split(recv, " ")[0] == "sql" {
		tcp_api.Execute_sql(safe_conn, recv, Global_database_net)
		return
	}
	safe_conn.Lock.Lock()
	safe_conn.Conn.Write([]byte("received: " + recv))
	safe_conn.Lock.Unlock()
}

func process_tcp(safe_conn utils.Safe_connection) {
	//关闭连接
	for {
		reader := bufio.NewReader(safe_conn.Conn)
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

	safe_conn.Lock.Lock()
	safe_conn.Conn.Write([]byte("server close"))
	_ = safe_conn.Conn.Close()
	safe_conn.Lock.Unlock()

	for i, v := range Global_conn_list {
		if v == &safe_conn {
			Global_conn_list = append(Global_conn_list[:i], Global_conn_list[i+1:]...)
			v.Lock = nil
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
	listen := utils.Retry_task(task_net_listen, Globalsig_ss, "127.0.0.1:50021").(net.Listener)

	for {
		if Globalsig_ss == 0 {
			// close all connection
			Global_conn_list_lock.Lock()
			for _, v := range Global_conn_list {
				v.Lock.Lock()
				v.Conn.Write([]byte("server close"))
				_ = v.Conn.Close()
				v.Lock.Unlock()
				v.Lock = nil
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
		safe_conn := utils.Safe_connection{Conn: conn, Lock: &conn_lock}
		Global_conn_list = append(Global_conn_list, &safe_conn)
		//start goroutine processs
		go process_tcp(safe_conn)
	}

}
