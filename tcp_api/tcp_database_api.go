package tcp_api

import (
	"fmt"
	"os"
	"screenshot_server/Global"
	"screenshot_server/utils"
	"sort"
	"strconv"
	"strings"
)

func query_database_count() (int, error) {
	var count int
	err := Global.Global_database_net.QueryRow("SELECT count(*) FROM screenshots").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func query_database_date_count(date string) (int, error) {
	var count int
	date_struct := utils.Decode_dateTimeStr(date, Global.Globalsig_ss)
	err := Global.Global_database_net.QueryRow("SELECT count(*) FROM screenshots WHERE year=? AND month=? AND day=?", date_struct.Year, date_struct.Month, date_struct.Day).Scan(&count)
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
	hour_int := utils.Retry_task(task_strconv_atoi, Global.Globalsig_ss, hour).(int)
	err := Global.Global_database_net.QueryRow("SELECT count(*) FROM screenshots WHERE hour=?", strconv.Itoa(hour_int)).Scan(&count)
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
		count := utils.Retry_task(task_query_database_hour_count, Global.Globalsig_ss, strconv.Itoa(hour_int)).(int)
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
	rows, err := Global.Global_database_net.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
	if err := rows.Err(); err != nil {
		return nil, err
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
	err := Global.Global_database_net.QueryRow("SELECT MIN(YEAR || '-' || printf('%02d', MONTH) || '-' || printf('%02d', DAY)) AS min_date FROM screenshots").Scan(&date)
	if err != nil {
		return "", err
	}
	date = strings.Replace(date, "-", "", -1)
	return date, nil
}

func query_max_date() (string, error) {
	var date string
	err := Global.Global_database_net.QueryRow("SELECT MAX(YEAR || '-' || printf('%02d', MONTH) || '-' || printf('%02d', DAY)) AS max_date FROM screenshots").Scan(&date)
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
	hour_int := utils.Retry_task(task_strconv_atoi, Global.Globalsig_ss, hour).(int)
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
	rows, err := Global.Global_database_net.Query(query, strconv.Itoa(hour_int))
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
	if err := rows.Err(); err != nil {
		return nil, err
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

func query_database_date_hour_count_all(date string) (map[string]int, error) {
	date_struct := utils.Decode_dateTimeStr(date, Global.Globalsig_ss)
	query := `
		SELECT 
			HOUR
		FROM 
			screenshots
		WHERE 
			YEAR = ? AND MONTH = ? AND DAY = ?
	`
	rows, err := Global.Global_database_net.Query(query, strconv.Itoa(date_struct.Year), strconv.Itoa(date_struct.Month), strconv.Itoa(date_struct.Day))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make(map[string]int)
	for rows.Next() {
		var hour int
		err = rows.Scan(&hour)
		if err != nil {
			return nil, err
		}
		if res[strconv.Itoa(hour)] == 0 {
			res[strconv.Itoa(hour)] = 1
		} else {
			res[strconv.Itoa(hour)]++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func query_database_date_hour_count(date string, hour string) (int, error) {
	date_struct := utils.Decode_dateTimeStr(date, Global.Globalsig_ss)
	task_strconv_atoi := func(args ...interface{}) (interface{}, error) {
		return strconv.Atoi(args[0].(string))
	}
	hour_int := utils.Retry_task(task_strconv_atoi, Global.Globalsig_ss, hour).(int)

	var count int
	err := Global.Global_database_net.QueryRow(
		"SELECT count(*) FROM screenshots WHERE year=? AND month=? AND day=? AND hour=?",
		date_struct.Year,
		date_struct.Month,
		date_struct.Day,
		hour_int,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func query_database_date_filename(date string) ([]string, error) {
	date_struct := utils.Decode_dateTimeStr(date, Global.Globalsig_ss)
	query := `
		SELECT 
			FILE_NAME
		FROM 
			screenshots
		WHERE 
			YEAR = ? AND MONTH = ? AND DAY = ?
	`
	rows, err := Global.Global_database_net.Query(query, strconv.Itoa(date_struct.Year), strconv.Itoa(date_struct.Month), strconv.Itoa(date_struct.Day))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make([]string, 0)
	for rows.Next() {
		var filename string
		err = rows.Scan(&filename)
		if err != nil {
			return nil, err
		}
		res = append(res, filename)
	}
	return res, nil
}

func query_database_hour_filename(hour string) ([]string, error) {
	task_strconv_atoi := func(args ...interface{}) (interface{}, error) {
		return strconv.Atoi(args[0].(string))
	}
	hour_int := utils.Retry_task(task_strconv_atoi, Global.Globalsig_ss, hour).(int)
	query := `
		SELECT 
			FILE_NAME
		FROM 
			screenshots
		WHERE 
			HOUR = ?
	`
	rows, err := Global.Global_database_net.Query(query, strconv.Itoa(hour_int))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make([]string, 0)
	for rows.Next() {
		var filename string
		err = rows.Scan(&filename)
		if err != nil {
			return nil, err
		}
		res = append(res, filename)
	}
	return res, nil
}

func query_database_date_hour_filename(date string, hour string) ([]string, error) {
	task_strconv_atoi := func(args ...interface{}) (interface{}, error) {
		return strconv.Atoi(args[0].(string))
	}
	hour_int := utils.Retry_task(task_strconv_atoi, Global.Globalsig_ss, hour).(int)
	date_struct := utils.Decode_dateTimeStr(date, Global.Globalsig_ss)
	query := `
		SELECT 
			FILE_NAME
		FROM 
			screenshots
		WHERE 
			YEAR = ? AND MONTH = ? AND DAY = ? AND HOUR = ?
	`
	rows, err := Global.Global_database_net.Query(query, strconv.Itoa(date_struct.Year), strconv.Itoa(date_struct.Month), strconv.Itoa(date_struct.Day), strconv.Itoa(hour_int))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make([]string, 0)
	for rows.Next() {
		var filename string
		err = rows.Scan(&filename)
		if err != nil {
			return nil, err
		}
		res = append(res, filename)
	}
	return res, nil
}

func writeSQLResponse(safe_conn utils.Safe_connection, message string) {
	safe_conn.Lock.Lock()
	safe_conn.Conn.Write([]byte(message))
	safe_conn.Lock.Unlock()
}

func validateCountDateArg(date string) (string, error) {
	if len(date) != 8 {
		return "", fmt.Errorf("invalid date format")
	}
	if _, err := strconv.Atoi(date); err != nil {
		return "", fmt.Errorf("invalid date format")
	}
	return date, nil
}

func validateCountHourArg(hour string) (string, error) {
	if len(hour) == 0 || len(hour) > 2 {
		return "", fmt.Errorf("invalid hour format")
	}
	hourInt, err := strconv.Atoi(hour)
	if err != nil || hourInt < 0 || hourInt > 23 {
		return "", fmt.Errorf("invalid hour format")
	}
	return strconv.Itoa(hourInt), nil
}

func formatDateCounts(res map[string]int) string {
	var keys []string
	for k := range res {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, date := range keys {
		builder.WriteString("\ndate ")
		builder.WriteString(date)
		builder.WriteString(": ")
		builder.WriteString(strconv.Itoa(res[date]))
	}
	return builder.String()
}

func formatHourCounts(res map[string]int) string {
	var builder strings.Builder
	for hourInt := 0; hourInt < 24; hourInt++ {
		hour := strconv.Itoa(hourInt)
		builder.WriteString("\nhour ")
		builder.WriteString(hour)
		builder.WriteString(": ")
		builder.WriteString(strconv.Itoa(res[hour]))
	}
	return builder.String()
}

func execute_sql_count(safe_conn utils.Safe_connection, recv_list []string) {
	if len(recv_list) == 0 {
		taskQueryDatabaseCount := func(args ...interface{}) (interface{}, error) {
			return query_database_count()
		}
		count := utils.Retry_task(taskQueryDatabaseCount, Global.Globalsig_ss).(int)
		writeSQLResponse(safe_conn, "total data count: "+strconv.Itoa(count))
		return
	}

	if len(recv_list) == 2 && recv_list[0] == "date" && recv_list[1] == "all" {
		taskQueryDatabaseDateCountAll := func(args ...interface{}) (interface{}, error) {
			return query_database_date_count_all()
		}
		res := utils.Retry_task(taskQueryDatabaseDateCountAll, Global.Globalsig_ss).(map[string]int)
		writeSQLResponse(safe_conn, formatDateCounts(res))
		return
	}
	if len(recv_list) == 2 && recv_list[0] == "date" {
		date, err := validateCountDateArg(recv_list[1])
		if err != nil {
			writeSQLResponse(safe_conn, err.Error())
			return
		}
		taskQueryDatabaseDateCount := func(args ...interface{}) (interface{}, error) {
			return query_database_date_count(args[0].(string))
		}
		count := utils.Retry_task(taskQueryDatabaseDateCount, Global.Globalsig_ss, date).(int)
		writeSQLResponse(safe_conn, "total data count: "+strconv.Itoa(count))
		return
	}

	if len(recv_list) == 2 && recv_list[0] == "hour" && recv_list[1] == "all" {
		res := query_database_hour_count_all()
		writeSQLResponse(safe_conn, formatHourCounts(res))
		return
	}
	if len(recv_list) == 2 && recv_list[0] == "hour" {
		hour, err := validateCountHourArg(recv_list[1])
		if err != nil {
			writeSQLResponse(safe_conn, err.Error())
			return
		}
		taskQueryDatabaseHourCount := func(args ...interface{}) (interface{}, error) {
			return query_database_hour_count(args[0].(string))
		}
		count := utils.Retry_task(taskQueryDatabaseHourCount, Global.Globalsig_ss, hour).(int)
		writeSQLResponse(safe_conn, "total data count: "+strconv.Itoa(count))
		return
	}

	if len(recv_list) == 4 && recv_list[0] == "date" && recv_list[2] == "hour" && recv_list[3] == "all" {
		date, err := validateCountDateArg(recv_list[1])
		if err != nil {
			writeSQLResponse(safe_conn, err.Error())
			return
		}
		taskQueryDatabaseDateHourCountAll := func(args ...interface{}) (interface{}, error) {
			return query_database_date_hour_count_all(args[0].(string))
		}
		res := utils.Retry_task(taskQueryDatabaseDateHourCountAll, Global.Globalsig_ss, date).(map[string]int)
		writeSQLResponse(safe_conn, formatHourCounts(res))
		return
	}

	if len(recv_list) == 4 && recv_list[0] == "hour" && recv_list[2] == "date" && recv_list[3] == "all" {
		hour, err := validateCountHourArg(recv_list[1])
		if err != nil {
			writeSQLResponse(safe_conn, err.Error())
			return
		}
		taskQueryDatabaseHourDateCountAll := func(args ...interface{}) (interface{}, error) {
			return query_database_hour_date_count_all(args[0].(string))
		}
		res := utils.Retry_task(taskQueryDatabaseHourDateCountAll, Global.Globalsig_ss, hour).(map[string]int)
		writeSQLResponse(safe_conn, formatDateCounts(res))
		return
	}

	if len(recv_list) == 4 && recv_list[0] == "date" && recv_list[2] == "hour" {
		date, dateErr := validateCountDateArg(recv_list[1])
		if dateErr != nil {
			writeSQLResponse(safe_conn, dateErr.Error())
			return
		}
		hour, hourErr := validateCountHourArg(recv_list[3])
		if hourErr != nil {
			writeSQLResponse(safe_conn, hourErr.Error())
			return
		}
		taskQueryDatabaseDateHourCount := func(args ...interface{}) (interface{}, error) {
			return query_database_date_hour_count(args[0].(string), args[1].(string))
		}
		count := utils.Retry_task(taskQueryDatabaseDateHourCount, Global.Globalsig_ss, date, hour).(int)
		writeSQLResponse(safe_conn, "total data count: "+strconv.Itoa(count))
		return
	}

	writeSQLResponse(safe_conn, "invalid sql count command")
}

func execute_sql_dump_count(safe_conn utils.Safe_connection, recv_list []string, recv string) {
	if len(recv_list) == 0 {
		task_query_database_count := func(args ...interface{}) (interface{}, error) {
			return query_database_count()
		}

		count := utils.Retry_task(task_query_database_count, Global.Globalsig_ss).(int)

		go func() {
			task_os_create := func(args ...interface{}) (interface{}, error) {
				file, err := os.Create(args[0].(string))
				return file, err
			}
			currentTime := utils.GetDatetime()
			file_name := Global.Global_constant_config.Dump_path + "/" + currentTime + "_dump.txt"
			file := utils.Retry_task(task_os_create, Global.Globalsig_ss, file_name).(*os.File)
			defer file.Close()
			file.Write([]byte("command executed: " + recv + "\n"))
			file.Write([]byte("total data count: " + strconv.Itoa(count)))

			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Target results dumped."))
			safe_conn.Lock.Unlock()
		}()
		return
	}
	if recv_list[0] == "date" && recv_list[1] != "all" && len(recv_list) == 2 {
		if len(recv_list[1]) != 8 {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("invalid date format"))
			safe_conn.Lock.Unlock()
			return
		}
		_, err := strconv.Atoi(recv_list[1])
		if err != nil {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("invalid date format"))
			safe_conn.Lock.Unlock()
			return
		}
		task_query_database_date_count := func(args ...interface{}) (interface{}, error) {
			return query_database_date_count(args[0].(string))
		}
		count := utils.Retry_task(task_query_database_date_count, Global.Globalsig_ss, recv_list[1]).(int)

		go func() {
			task_os_create := func(args ...interface{}) (interface{}, error) {
				file, err := os.Create(args[0].(string))
				return file, err
			}
			currentTime := utils.GetDatetime()
			file_name := Global.Global_constant_config.Dump_path + "/" + currentTime + "_dump.txt"
			file := utils.Retry_task(task_os_create, Global.Globalsig_ss, file_name).(*os.File)
			defer file.Close()
			file.Write([]byte("command executed: " + recv + "\n"))
			file.Write([]byte("total data count: " + strconv.Itoa(count)))

			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Target results dumped."))
			safe_conn.Lock.Unlock()
		}()
		return
	}
	if recv_list[0] == "date" && recv_list[1] == "all" && len(recv_list) == 2 {
		task_query_database_date_count_all := func(args ...interface{}) (interface{}, error) {
			return query_database_date_count_all()
		}
		res := utils.Retry_task(task_query_database_date_count_all, Global.Globalsig_ss).(map[string]int)

		go func() {
			task_os_create := func(args ...interface{}) (interface{}, error) {
				file, err := os.Create(args[0].(string))
				return file, err
			}
			currentTime := utils.GetDatetime()
			file_name := Global.Global_constant_config.Dump_path + "/" + currentTime + "_dump.txt"
			file := utils.Retry_task(task_os_create, Global.Globalsig_ss, file_name).(*os.File)
			defer file.Close()

			var keys []string
			for k := range res {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, date := range keys {
				file.Write([]byte("date " + date + ": " + strconv.Itoa(res[date]) + "\n"))
			}

			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Target results dumped."))
			safe_conn.Lock.Unlock()
		}()

		return
	}
	if recv_list[0] == "hour" && recv_list[1] != "all" && len(recv_list) == 2 {
		if len(recv_list[1]) > 2 {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("invalid hour format"))
			safe_conn.Lock.Unlock()
			return
		}
		_, err := strconv.Atoi(recv_list[3])
		if err != nil {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("invalid hour format"))
			safe_conn.Lock.Unlock()
			return
		}
		task_query_database_hour_count := func(args ...interface{}) (interface{}, error) {
			return query_database_hour_count(args[0].(string))
		}
		count := utils.Retry_task(task_query_database_hour_count, Global.Globalsig_ss, recv_list[1]).(int)

		go func() {
			task_os_create := func(args ...interface{}) (interface{}, error) {
				file, err := os.Create(args[0].(string))
				return file, err
			}
			currentTime := utils.GetDatetime()
			file_name := Global.Global_constant_config.Dump_path + "/" + currentTime + "_dump.txt"
			file := utils.Retry_task(task_os_create, Global.Globalsig_ss, file_name).(*os.File)
			defer file.Close()
			file.Write([]byte("command executed: " + recv + "\n"))
			file.Write([]byte("total data count: " + strconv.Itoa(count)))

			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Target results dumped."))
			safe_conn.Lock.Unlock()
		}()

		return
	}
	if recv_list[0] == "hour" && recv_list[1] == "all" && len(recv_list) == 2 {
		res := query_database_hour_count_all()

		go func() {
			task_os_create := func(args ...interface{}) (interface{}, error) {
				file, err := os.Create(args[0].(string))
				return file, err
			}
			currentTime := utils.GetDatetime()
			file_name := Global.Global_constant_config.Dump_path + "/" + currentTime + "_dump.txt"
			file := utils.Retry_task(task_os_create, Global.Globalsig_ss, file_name).(*os.File)
			defer file.Close()

			file.Write([]byte("command executed: " + recv + "\n"))
			for hour_int := 0; hour_int < 24; hour_int++ {
				file.Write([]byte("hour " + strconv.Itoa(hour_int) + ": " + strconv.Itoa(res[strconv.Itoa(hour_int)]) + "\n"))
			}

			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Target results dumped."))
			safe_conn.Lock.Unlock()
		}()

		return
	}
	if recv_list[0] == "date" && recv_list[1] == "hour" && recv_list[2] == "all" && len(recv_list) == 4 {
		if len(recv_list[3]) != 8 {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("invalid date format"))
			safe_conn.Lock.Unlock()
			return
		}
		_, err := strconv.Atoi(recv_list[3])
		if err != nil {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("invalid date format"))
			safe_conn.Lock.Unlock()
			return
		}
		task_query_database_date_hour_count_all := func(args ...interface{}) (interface{}, error) {
			return query_database_date_hour_count_all(args[0].(string))
		}
		res := utils.Retry_task(task_query_database_date_hour_count_all, Global.Globalsig_ss, recv_list[3]).(map[string]int)

		go func() {
			task_os_create := func(args ...interface{}) (interface{}, error) {
				file, err := os.Create(args[0].(string))
				return file, err
			}
			currentTime := utils.GetDatetime()
			file_name := Global.Global_constant_config.Dump_path + "/" + currentTime + "_dump.txt"
			file := utils.Retry_task(task_os_create, Global.Globalsig_ss, file_name).(*os.File)
			defer file.Close()

			file.Write([]byte("command executed: " + recv + "\n"))
			for hour_int := 0; hour_int < 24; hour_int++ {
				file.Write([]byte("hour " + strconv.Itoa(hour_int) + ": " + strconv.Itoa(res[strconv.Itoa(hour_int)]) + "\n"))
			}

			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Target results dumped."))
			safe_conn.Lock.Unlock()
		}()

		return
	}
	if recv_list[0] == "hour" && recv_list[1] == "date" && recv_list[2] == "all" && len(recv_list) == 4 {
		if len(recv_list[3]) > 2 {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("invalid hour format"))
			safe_conn.Lock.Unlock()
			return
		}
		_, err := strconv.Atoi(recv_list[3])
		if err != nil {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("invalid hour format"))
			safe_conn.Lock.Unlock()
			return
		}
		task_strconv_atoi := func(args ...interface{}) (interface{}, error) {
			return strconv.Atoi(args[0].(string))
		}
		task_query_database_hour_date_all := func(args ...interface{}) (interface{}, error) {
			return query_database_hour_date_count_all(args[0].(string))
		}
		res := utils.Retry_task(task_query_database_hour_date_all, Global.Globalsig_ss, recv_list[3]).(map[string]int)

		go func() {
			task_os_create := func(args ...interface{}) (interface{}, error) {
				file, err := os.Create(args[0].(string))
				return file, err
			}
			currentTime := utils.GetDatetime()
			file_name := Global.Global_constant_config.Dump_path + "/" + currentTime + "_dump.txt"
			file := utils.Retry_task(task_os_create, Global.Globalsig_ss, file_name).(*os.File)
			defer file.Close()

			var keys []string
			for k := range res {
				keys = append(keys, k)
			}
			var keys_int []int
			for _, k := range keys {
				keys_int = append(keys_int, utils.Retry_task(task_strconv_atoi, Global.Globalsig_ss, k).(int))
			}
			sort.Ints(keys_int)
			for i, k := range keys_int {
				keys[i] = strconv.Itoa(k)
			}
			file.Write([]byte("command executed: " + recv + "\n"))
			for _, date := range keys {
				file.Write([]byte("date " + date + ": " + strconv.Itoa(res[date]) + "\n"))
			}

			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Target results dumped."))
			safe_conn.Lock.Unlock()
		}()

		return
	}
	safe_conn.Lock.Lock()
	safe_conn.Conn.Write([]byte("Invalid sql dump count command"))
	safe_conn.Lock.Unlock()
}

func execute_sql_dump_filename(safe_conn utils.Safe_connection, recv_list []string, recv string) {
	if len(recv_list) == 0 {
		safe_conn.Lock.Lock()
		// safe_conn.Conn.Write([]byte("Target results dumped."))
		safe_conn.Conn.Write([]byte("Invalid sql dump filename command"))
		safe_conn.Lock.Unlock()
		return
	}
	if len(recv_list) == 2 && utils.In_string_list("hour", recv_list) && !utils.In_string_list("date", recv_list) {
		if len(recv_list[1]) > 2 {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Invalid hour format"))
			safe_conn.Lock.Unlock()
			return
		}
		_, err := strconv.Atoi(recv_list[1])
		if err != nil {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Invalid hour format"))
			safe_conn.Lock.Unlock()
			return
		}

		task_query_database_hour_filename := func(args ...interface{}) (interface{}, error) {
			return query_database_hour_filename(args[0].(string))
		}
		res := utils.Retry_task(task_query_database_hour_filename, Global.Globalsig_ss, recv_list[1]).([]string)
		go func() {
			task_os_create := func(args ...interface{}) (interface{}, error) {
				file, err := os.Create(args[0].(string))
				return file, err
			}
			currentTime := utils.GetDatetime()
			file_name := Global.Global_constant_config.Dump_path + "/" + currentTime + "_dump.txt"
			file := utils.Retry_task(task_os_create, Global.Globalsig_ss, file_name).(*os.File)
			defer file.Close()
			file.Write([]byte("command executed: " + recv + "\n"))
			sort.Strings(res)
			for _, filename := range res {
				file.Write([]byte(filename + "\n"))
			}

			safe_conn.Lock.Lock()
			// safe_conn.Conn.Write([]byte("Target results dumped."))
			safe_conn.Conn.Write([]byte("Target results dumped."))
			safe_conn.Lock.Unlock()
		}()
		return
	}
	if len(recv_list) == 2 && !utils.In_string_list("hour", recv_list) && utils.In_string_list("date", recv_list) {
		if len(recv_list[1]) != 8 {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Invalid hour format"))
			safe_conn.Lock.Unlock()
			return
		}
		_, err := strconv.Atoi(recv_list[1])
		if err != nil {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Invalid date format"))
			safe_conn.Lock.Unlock()
			return
		}

		task_query_database_date_filename := func(args ...interface{}) (interface{}, error) {
			return query_database_date_filename(args[0].(string))
		}
		res := utils.Retry_task(task_query_database_date_filename, Global.Globalsig_ss, recv_list[1]).([]string)
		go func() {
			task_os_create := func(args ...interface{}) (interface{}, error) {
				file, err := os.Create(args[0].(string))
				return file, err
			}
			currentTime := utils.GetDatetime()
			file_name := Global.Global_constant_config.Dump_path + "/" + currentTime + "_dump.txt"
			file := utils.Retry_task(task_os_create, Global.Globalsig_ss, file_name).(*os.File)
			defer file.Close()
			file.Write([]byte("command executed: " + recv + "\n"))
			sort.Strings(res)
			for _, filename := range res {
				file.Write([]byte(filename + "\n"))
			}

			safe_conn.Lock.Lock()
			// safe_conn.Conn.Write([]byte("Target results dumped."))
			safe_conn.Conn.Write([]byte("Target results dumped."))
			safe_conn.Lock.Unlock()
		}()
		return
	}
	if len(recv_list) == 4 && utils.In_string_list("hour", recv_list) && utils.In_string_list("date", recv_list) {
		index_hour := utils.In_string_list_index("hour", recv_list)
		index_date := utils.In_string_list_index("date", recv_list)
		fmt.Println(index_hour, index_date)
		if !((index_hour == 0 && index_date == 2) || (index_hour == 2 && index_date == 0)) {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Invalid sql dump filename command"))
			safe_conn.Lock.Unlock()
			return
		}
		if len(recv_list[index_hour+1]) > 2 {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Invalid hour format"))
			safe_conn.Lock.Unlock()
			return
		}
		_, err := strconv.Atoi(recv_list[index_hour+1])
		if err != nil {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Invalid hour format"))
			safe_conn.Lock.Unlock()
			return
		}
		if len(recv_list[index_date+1]) != 8 {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Invalid date format"))
			safe_conn.Lock.Unlock()
			return
		}
		_, err = strconv.Atoi(recv_list[index_date+1])
		if err != nil {
			safe_conn.Lock.Lock()
			safe_conn.Conn.Write([]byte("Invalid date format"))
			safe_conn.Lock.Unlock()
			return
		}
		hour_string := recv_list[index_hour+1]
		date_string := recv_list[index_date+1]

		task_query_database_date_hour_filename := func(args ...interface{}) (interface{}, error) {
			return query_database_date_hour_filename(args[0].(string), args[1].(string))
		}
		res := utils.Retry_task(task_query_database_date_hour_filename, Global.Globalsig_ss, date_string, hour_string).([]string)
		go func() {
			task_os_create := func(args ...interface{}) (interface{}, error) {
				file, err := os.Create(args[0].(string))
				return file, err
			}
			currentTime := utils.GetDatetime()
			file_name := Global.Global_constant_config.Dump_path + "/" + currentTime + "_dump.txt"
			file := utils.Retry_task(task_os_create, Global.Globalsig_ss, file_name).(*os.File)
			defer file.Close()
			file.Write([]byte("command executed: " + recv + "\n"))
			sort.Strings(res)
			for _, filename := range res {
				file.Write([]byte(filename + "\n"))
			}

			safe_conn.Lock.Lock()
			// safe_conn.Conn.Write([]byte("Target results dumped."))
			safe_conn.Conn.Write([]byte("Target results dumped."))
			safe_conn.Lock.Unlock()
		}()
		return
	}
	safe_conn.Lock.Lock()
	safe_conn.Conn.Write([]byte("Invalid sql dump filename command"))
	safe_conn.Lock.Unlock()

}

func execute_sql_dump(safe_conn utils.Safe_connection, recv_list []string, recv string) {
	if len(recv_list) == 0 {
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("invalid sql dump command"))
		safe_conn.Lock.Unlock()
		return
	}
	if recv_list[0] == "count" {
		execute_sql_dump_count(safe_conn, recv_list[1:], recv)
		return
	}
	if recv_list[0] == "filename" {
		execute_sql_dump_filename(safe_conn, recv_list[1:], recv)
		return
	}
	safe_conn.Lock.Lock()
	safe_conn.Conn.Write([]byte("invalid sql dump command"))
	safe_conn.Lock.Unlock()
}

func Execute_sql(safe_conn utils.Safe_connection, recv string) {
	recv_list := strings.Fields(recv)
	if len(recv_list) < 2 { // important!
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("invalid sql command"))
		safe_conn.Lock.Unlock()
		return
	}
	if recv_list[1] == "count" {
		execute_sql_count(safe_conn, recv_list[2:])
		return
	}
	if recv_list[1] == "dump" {
		execute_sql_dump(safe_conn, recv_list[2:], recv)
		return
	}
	if recv_list[1] == "min_date" {
		task_query_min_date := func(args ...interface{}) (interface{}, error) {
			return query_min_date()
		}
		date := utils.Retry_task(task_query_min_date, Global.Globalsig_ss).(string)
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("min date: " + date))
		safe_conn.Lock.Unlock()
		return
	}
	if recv_list[1] == "max_date" {
		task_query_max_date := func(args ...interface{}) (interface{}, error) {
			return query_max_date()
		}
		date := utils.Retry_task(task_query_max_date, Global.Globalsig_ss).(string)
		safe_conn.Lock.Lock()
		safe_conn.Conn.Write([]byte("max date: " + date))
		safe_conn.Lock.Unlock()
		return
	}
	safe_conn.Lock.Lock()
	safe_conn.Conn.Write([]byte("invalid sql command"))
	safe_conn.Lock.Unlock()
}
