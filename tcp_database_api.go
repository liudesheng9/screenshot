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

func query_database_hour_count(hour string) (int, error) {
	var count int
	task_strconv_atoi := func(args ...interface{}) (interface{}, error) {
		return strconv.Atoi(args[0].(string))
	}
	hour_int := retry_task(task_strconv_atoi, hour).(int)
	err := Global_database_net.QueryRow("SELECT count(*) FROM screenshots WHERE hour=?", strconv.Itoa(hour_int)).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func query_database_hour_count_all() map[string]int {
	task_query_database_hour_count := func(args ...interface{}) (interface{}, error) {
		return query_database_hour_count(args[0].(string))
	}
	res := make(map[string]int)
	for hour_int := 0; hour_int < 24; hour_int++ {
		count := retry_task(task_query_database_hour_count, strconv.Itoa(hour_int)).(int)
		res[strconv.Itoa(hour_int)] = count
	}
	return res
}

func query_min_date() (string, error) {
	var date string
	err := Global_database_net.QueryRow("SELECT MIN(YEAR || '-' || printf('%02d', MONTH) || '-' || printf('%02d', DAY)) AS min_date FROM screenshots").Scan(&date)
	if err != nil {
		return "", err
	}
	date = strings.Replace(date, "-", "", -1)
	return date, nil
}

func query_max_date() (string, error) {
	var date string
	err := Global_database_net.QueryRow("SELECT MAX(YEAR || '-' || printf('%02d', MONTH) || '-' || printf('%02d', DAY)) AS max_date FROM screenshots").Scan(&date)
	if err != nil {
		return "", err
	}
	date = strings.Replace(date, "-", "", -1)
	return date, nil
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
			return query_database_date_count(args[0].(string))
		}
		count := retry_task(task_query_database_date_count, recv_list[3]).(int)
		safe_conn.lock.Lock()
		safe_conn.conn.Write([]byte("total data count: " + strconv.Itoa(count)))
		safe_conn.lock.Unlock()
		return
	}
	if recv_list[1] == "count" && recv_list[2] == "hour" && recv_list[3] != "all" && len(recv_list) == 4 {
		if len(recv_list[3]) > 2 {
			safe_conn.lock.Lock()
			safe_conn.conn.Write([]byte("invalid hour format"))
			safe_conn.lock.Unlock()
			return
		}
		task_query_database_hour_count := func(args ...interface{}) (interface{}, error) {
			return query_database_hour_count(args[0].(string))
		}
		count := retry_task(task_query_database_hour_count, recv_list[3]).(int)
		safe_conn.lock.Lock()
		safe_conn.conn.Write([]byte("total data count: " + strconv.Itoa(count)))
		safe_conn.lock.Unlock()
		return
	}
	if recv_list[1] == "count" && recv_list[2] == "hour" && recv_list[3] == "all" && len(recv_list) == 4 {
		res := query_database_hour_count_all()
		write_str := ""
		for hour_int := 0; hour_int < 24; hour_int++ {
			write_str += "\n" + "hour " + strconv.Itoa(hour_int) + ": " + strconv.Itoa(res[strconv.Itoa(hour_int)])
		}
		safe_conn.lock.Lock()
		safe_conn.conn.Write([]byte(write_str))
		safe_conn.lock.Unlock()
		return
	}
	if recv_list[1] == "min_date" {
		task_query_min_date := func(args ...interface{}) (interface{}, error) {
			return query_min_date()
		}
		date := retry_task(task_query_min_date).(string)
		safe_conn.lock.Lock()
		safe_conn.conn.Write([]byte("min date: " + date))
		safe_conn.lock.Unlock()
		return
	}
	if recv_list[1] == "max_date" {
		task_query_max_date := func(args ...interface{}) (interface{}, error) {
			return query_max_date()
		}
		date := retry_task(task_query_max_date).(string)
		safe_conn.lock.Lock()
		safe_conn.conn.Write([]byte("max date: " + date))
		safe_conn.lock.Unlock()
		return
	}
}
