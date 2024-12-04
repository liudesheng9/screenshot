package main

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"time"
)

func excute_recv_command(conn net.Conn, recv string) {
	if recv == "0" {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 0
		Global_sig_ss_Mutex.Unlock()
		conn.Write([]byte("set stop"))
		return
	}
	if recv == "1" {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 1
		Global_sig_ss_Mutex.Unlock()
		conn.Write([]byte("set start"))
		return
	}
	if recv == "2" {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 2
		Global_sig_ss_Mutex.Unlock()
		conn.Write([]byte("set pause"))
		return
	}
	if recv == "hello server" {
		conn.Write([]byte("1"))
		return
	}
	conn.Write([]byte("received: " + recv))
}

func process_tcp(conn net.Conn) {
	var conn_lock sync.Mutex

	//关闭连接
	for {
		reader := bufio.NewReader(conn)
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
			conn_lock.Lock()
			excute_recv_command(conn, recv)
			conn_lock.Unlock()
		}()
		fmt.Println("byte received: ", recv)
	}
	conn_lock.Lock()
	conn.Write([]byte("server close"))
	conn.Close()
	conn_lock.Unlock()
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
		//start goroutine processs
		go process_tcp(conn)
	}

}
