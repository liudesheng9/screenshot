package main

import (
	"strconv"
	"strings"
)

func query_database_count() (int, error) {
	var count int
	err := Global_database_net.QueryRow("SELECT count(*) FROM screenshots").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func execute_sql(safe_conn safe_connection, recv string) {
	recv_list := strings.Split(recv, " ")
	if recv_list[1] == "count" {
		task_query_database_count := func(args ...interface{}) (interface{}, error) {
			return query_database_count()
		}
		count := retry_task(task_query_database_count).(int)
		safe_conn.lock.Lock()
		safe_conn.conn.Write([]byte("total data count: " + strconv.Itoa(count)))
		safe_conn.lock.Unlock()
		return
	}
}
