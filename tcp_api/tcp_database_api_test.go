package tcp_api

import (
	"database/sql"
	"net"
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
			file_name TEXT
		)
	`)
	if err != nil {
		t.Fatalf("create screenshots table: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO screenshots(year, month, day, hour, minute, file_name) VALUES
			(2025, 1, 1, 10, 0, 'a.png'),
			(2025, 1, 1, 10, 30, 'b.png'),
			(2025, 1, 1, 11, 0, 'c.png'),
			(2025, 1, 2, 10, 15, 'd.png')
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
