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

func query_database_date_count(date string) (int, error) {
	var count int
	date_struct := decode_dateTimeStr(date)
	err := Global_database_net.QueryRow("SELECT count(*) FROM screenshots WHERE year=? AND month=? AND day=?", date_struct.year, date_struct.month, date_struct.day).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func execute_sql(safe_conn safe_connection, recv string) {
	recv_list := strings.Split(recv, " ")
	if recv_list[1] == "count" && len(recv_list) == 2 {
		task_query_database_count := func(args ...interface{}) (interface{}, error) {
			return query_database_count()
		}
		count := retry_task(task_query_database_count).(int)
		safe_conn.lock.Lock()
		safe_conn.conn.Write([]byte("total data count: " + strconv.Itoa(count)))
		safe_conn.lock.Unlock()
		return
	}
	if recv_list[1] == "count" && recv_list[2] == "date" && len(recv_list) == 4 {
		if len(recv_list[3]) != 8 {
			safe_conn.lock.Lock()
			safe_conn.conn.Write([]byte("invalid date format"))
			safe_conn.lock.Unlock()
			return
		}
		task_query_database_date_count := func(args ...interface{}) (interface{}, error) {
			return query_database_date_count(recv_list[3])
		}
		count := retry_task(task_query_database_date_count).(int)
		safe_conn.lock.Lock()
		safe_conn.conn.Write([]byte("total data count: " + strconv.Itoa(count)))
		safe_conn.lock.Unlock()
		return
	}
}
