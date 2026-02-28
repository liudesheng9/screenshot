package tcp_api

import (
	"database/sql"
	"net"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"screenshot_server/Global"
	"screenshot_server/utils"
)

func TestExecuteSQLCountCommands(t *testing.T) {
	db := createTestScreenshotsDB(t)
	defer db.Close()

	restoreGlobals := installSQLTestGlobals(db)
	defer restoreGlobals()

	testCases := []struct {
		name        string
		command     string
		wantContain []string
	}{
		{
			name:        "total_count",
			command:     "sql count",
			wantContain: []string{"total data count: 4"},
		},
		{
			name:        "count_by_date",
			command:     "sql count date 20250101",
			wantContain: []string{"total data count: 3"},
		},
		{
			name:        "count_by_date_all",
			command:     "sql count date all",
			wantContain: []string{"date 20250101: 3", "date 20250102: 1"},
		},
		{
			name:        "count_by_hour",
			command:     "sql count hour 10",
			wantContain: []string{"total data count: 3"},
		},
		{
			name:        "count_by_hour_all",
			command:     "sql count hour all",
			wantContain: []string{"hour 0: 0", "hour 10: 3", "hour 11: 1", "hour 23: 0"},
		},
		{
			name:        "count_by_date_hour_all",
			command:     "sql count date 20250101 hour all",
			wantContain: []string{"hour 0: 0", "hour 10: 2", "hour 11: 1", "hour 23: 0"},
		},
		{
			name:        "count_by_hour_date_all",
			command:     "sql count hour 10 date all",
			wantContain: []string{"date 20250101: 2", "date 20250102: 1"},
		},
		{
			name:        "count_by_date_and_hour",
			command:     "sql count date 20250101 hour 10",
			wantContain: []string{"total data count: 2"},
		},
		{
			name:        "count_by_machine_total",
			command:     "sql count --machine laptop1",
			wantContain: []string{"total data count: 3"},
		},
		{
			name:        "count_by_machine_date",
			command:     "sql count date 20250101 --machine laptop1",
			wantContain: []string{"total data count: 2"},
		},
		{
			name:        "count_by_machine_hour_all",
			command:     "sql count hour all --machine laptop1",
			wantContain: []string{"hour 10: 3", "hour 11: 0"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			output := runSQLCommand(t, tc.command)
			for _, expect := range tc.wantContain {
				if !strings.Contains(output, expect) {
					t.Fatalf("command %q output missing %q, got: %q", tc.command, expect, output)
				}
			}
		})
	}
}

func TestQueryDatabaseFilenameWithMachineFilter(t *testing.T) {
	db := createTestScreenshotsDB(t)
	defer db.Close()

	restoreGlobals := installSQLTestGlobals(db)
	defer restoreGlobals()

	byDate, err := query_database_date_filename("20250101", "laptop1")
	if err != nil {
		t.Fatalf("query_database_date_filename returned error: %v", err)
	}
	sort.Strings(byDate)
	if len(byDate) != 2 || byDate[0] != "a.png" || byDate[1] != "b.png" {
		t.Fatalf("unexpected machine-filtered date filenames: %+v", byDate)
	}

	byHour, err := query_database_hour_filename("10", "laptop1")
	if err != nil {
		t.Fatalf("query_database_hour_filename returned error: %v", err)
	}
	sort.Strings(byHour)
	if len(byHour) != 3 || byHour[0] != "a.png" || byHour[1] != "b.png" || byHour[2] != "d.png" {
		t.Fatalf("unexpected machine-filtered hour filenames: %+v", byHour)
	}

	byDateHour, err := query_database_date_hour_filename("20250101", "10", "laptop1")
	if err != nil {
		t.Fatalf("query_database_date_hour_filename returned error: %v", err)
	}
	sort.Strings(byDateHour)
	if len(byDateHour) != 2 || byDateHour[0] != "a.png" || byDateHour[1] != "b.png" {
		t.Fatalf("unexpected machine-filtered date+hour filenames: %+v", byDateHour)
	}
}

func createTestScreenshotsDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE screenshots (
			year INTEGER,
			month INTEGER,
			day INTEGER,
			hour INTEGER,
			minute INTEGER,
			display_num INTEGER,
			file_name TEXT,
			machine_id TEXT DEFAULT 'default'
		)
	`)
	if err != nil {
		t.Fatalf("create screenshots table: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO screenshots(year, month, day, hour, minute, display_num, file_name, machine_id) VALUES
			(2025, 1, 1, 10, 0, 1, 'a.png', 'laptop1'),
			(2025, 1, 1, 10, 30, 1, 'b.png', 'laptop1'),
			(2025, 1, 1, 11, 0, 2, 'c.png', 'desktop1'),
			(2025, 1, 2, 10, 15, 1, 'd.png', 'laptop1')
	`)
	if err != nil {
		t.Fatalf("insert screenshots fixtures: %v", err)
	}

	return db
}

func installSQLTestGlobals(db *sql.DB) func() {
	previousDB := Global.Global_database_net
	previousSig := Global.Globalsig_ss

	sig := 1
	Global.Global_database_net = db
	Global.Globalsig_ss = &sig

	return func() {
		Global.Global_database_net = previousDB
		Global.Globalsig_ss = previousSig
	}
}

func runSQLCommand(t *testing.T, command string) string {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	safeConn := utils.Safe_connection{
		Conn: serverConn,
		Lock: &sync.Mutex{},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		Execute_sql(safeConn, command)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 16384)
	n, err := clientConn.Read(buf)
	if err != nil {
		t.Fatalf("read sql response for %q: %v", command, err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for command completion: %q", command)
	}

	return string(buf[:n])
}
