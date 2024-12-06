package main

import (
	"sort"
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

func query_database_date_count_all() (map[string]int, error) {
	/*
		task_strconv_atoi := func(args ...interface{}) (interface{}, error) {
			return strconv.Atoi(args[0].(string))
		}
	*/
	query := `
		SELECT 
			YEAR || '-' || printf('%02d', MONTH) || '-' || printf('%02d', DAY) AS formatted_date
		FROM 
			screenshots
		WHERE
			YEAR IS NOT NULL
		ORDER BY 
			formatted_date;
	`
	rows, err := Global_database_net.Query(query)
	if err != nil {
		return nil, err
	}
	res := make(map[string]int)
	for rows.Next() {
		var formatted_date string
		err = rows.Scan(&formatted_date)
		formatted_date = strings.Replace(formatted_date, "-", "", -1)
		if err != nil {
			return nil, err
		}
		if res[formatted_date] == 0 {
			res[formatted_date] = 1
		} else {
			res[formatted_date]++
		}
	}
	// sort the map by key
	/*
		var keys []string
		for k := range res {
			keys = append(keys, k)
		}
		keys_int := make([]int, len(keys))
		for i, k := range keys {
			keys_int[i] = retry_task(task_strconv_atoi, k).(int)
		}
		sort.Ints(keys_int)
		fmt.Println(keys_int)
		for i, k := range keys_int {
			keys[i] = strconv.Itoa(k)
		}
		fmt.Println(keys)
		res_sorted := make(map[string]int)
		for _, k := range keys {
			res_sorted[k] = res[k]
		}
		fmt.Println(res_sorted)
		return res_sorted, nil
	*/
	return res, nil
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

func query_database_hour_date_count_all(hour string) (map[string]int, error) {
	task_strconv_atoi := func(args ...interface{}) (interface{}, error) {
		return strconv.Atoi(args[0].(string))
	}
	hour_int := retry_task(task_strconv_atoi, hour).(int)
	query := `
		SELECT 
			YEAR || '-' || printf('%02d', MONTH) || '-' || printf('%02d', DAY) AS formatted_date,
			HOUR
		FROM 
			screenshots
		WHERE 
			HOUR = ?
		ORDER BY 
			formatted_date;
	`
	rows, err := Global_database_net.Query(query, strconv.Itoa(hour_int))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make(map[string]int)
	for rows.Next() {
		var formatted_date string
		var hour int
		err = rows.Scan(&formatted_date, &hour)
		formatted_date = strings.Replace(formatted_date, "-", "", -1)
		if err != nil {
			return nil, err
		}
		if res[formatted_date] == 0 {
			res[formatted_date] = 1
		} else {
			res[formatted_date]++
		}
	}
	/*
		// sort the map by key
		var keys []string
		for k := range res {
			keys = append(keys, k)
		}
		keys_int := make([]int, len(keys))
		for i, k := range keys {
			keys_int[i] = retry_task(task_strconv_atoi, k).(int)
		}
		sort.Ints(keys_int)
		for i, k := range keys_int {
			keys[i] = strconv.Itoa(k)
		}
		res_sorted := make(map[string]int)
		for _, k := range keys {
			res_sorted[k] = res[k]
		}
		return res_sorted, nil
	*/
	return res, nil
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
	if recv_list[1] == "count" && recv_list[2] == "date" && recv_list[3] != "all" && len(recv_list) == 4 {
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
	if recv_list[1] == "count" && recv_list[2] == "date" && recv_list[3] == "all" && len(recv_list) == 4 {
		task_query_database_date_count_all := func(args ...interface{}) (interface{}, error) {
			return query_database_date_count_all()
		}
		res := retry_task(task_query_database_date_count_all).(map[string]int)
		write_str := ""
		var keys []string
		for k := range res {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, date := range keys {
			write_str += "\n" + "date " + date + ": " + strconv.Itoa(res[date])
		}
		safe_conn.lock.Lock()
		safe_conn.conn.Write([]byte(write_str))
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
	if recv_list[1] == "count" && recv_list[2] == "hour" && recv_list[3] == "date" && recv_list[4] == "all" && len(recv_list) == 6 {
		if len(recv_list[5]) > 2 {
			safe_conn.lock.Lock()
			safe_conn.conn.Write([]byte("invalid hour format"))
			safe_conn.lock.Unlock()
			return
		}
		task_query_database_hour_date_all := func(args ...interface{}) (interface{}, error) {
			return query_database_hour_date_count_all(args[0].(string))
		}
		res := retry_task(task_query_database_hour_date_all, recv_list[5]).(map[string]int)
		write_str := ""
		var keys []string
		for k := range res {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, date := range keys {
			write_str += "\n" + "date " + date + ": " + strconv.Itoa(res[date])
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
	safe_conn.lock.Lock()
	safe_conn.conn.Write([]byte("invalid sql command"))
	safe_conn.lock.Unlock()
}