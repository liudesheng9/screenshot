package import_manager

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestParseRemapFlag(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		want      map[int]int
		wantError bool
	}{
		{
			name:  "empty_is_valid",
			input: "",
			want:  map[int]int{},
		},
		{
			name:  "single_mapping",
			input: "1:2",
			want:  map[int]int{1: 2},
		},
		{
			name:  "multiple_mappings",
			input: "1:2,2:3,10:11",
			want:  map[int]int{1: 2, 2: 3, 10: 11},
		},
		{
			name:      "invalid_pair",
			input:     "1-2",
			wantError: true,
		},
		{
			name:      "invalid_number",
			input:     "1:x",
			wantError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRemapFlag(tc.input)
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRemapFlag returned error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("mapping size mismatch: got=%d want=%d", len(got), len(tc.want))
			}
			for src, dst := range tc.want {
				if got[src] != dst {
					t.Fatalf("mapping mismatch for %d: got=%d want=%d", src, got[src], dst)
				}
			}
		})
	}
}

func TestApplyRemap(t *testing.T) {
	remap := map[int]int{1: 2, 2: 3}
	if got := applyRemap(1, remap); got != 2 {
		t.Fatalf("expected remapped value 2, got %d", got)
	}
	if got := applyRemap(9, remap); got != 9 {
		t.Fatalf("expected unchanged value 9, got %d", got)
	}
}

func TestShouldInsertOrUpdate(t *testing.T) {
	fileID := "abc"

	if got := shouldInsertOrUpdate(fileID, true, "abc"); got != dedupSkip {
		t.Fatalf("expected skip when id exists, got %s", got)
	}
	if got := shouldInsertOrUpdate(fileID, false, "legacy"); got != dedupUpdate {
		t.Fatalf("expected update for filename collision, got %s", got)
	}
	if got := shouldInsertOrUpdate(fileID, false, ""); got != dedupInsert {
		t.Fatalf("expected insert for new file, got %s", got)
	}
}

func TestCheckExists(t *testing.T) {
	db := createImportManagerTestDB(t)
	defer db.Close()

	fileName := "exists.png"
	fileID := hashStringSHA256(fileName)
	_, err := db.Exec(
		`INSERT INTO screenshots (id, file_name) VALUES (?, ?)`,
		fileID,
		fileName,
	)
	if err != nil {
		t.Fatalf("insert fixture: %v", err)
	}

	exists, existingID, err := checkExists(db, fileName)
	if err != nil {
		t.Fatalf("checkExists returned error: %v", err)
	}
	if !exists {
		t.Fatalf("expected exists=true")
	}
	if existingID != fileID {
		t.Fatalf("expected existing id %s, got %s", fileID, existingID)
	}

	exists, existingID, err = checkExists(db, "missing.png")
	if err != nil {
		t.Fatalf("checkExists returned error for missing file: %v", err)
	}
	if exists {
		t.Fatalf("expected exists=false for missing file")
	}
	if existingID != "" {
		t.Fatalf("expected empty existingID for missing file, got %s", existingID)
	}
}

func createImportManagerTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE screenshots (
			id TEXT PRIMARY KEY NOT NULL,
			hash TEXT NULL,
			hash_kind TEXT NULL,
			year INT NULL,
			month INT NULL,
			day INT NULL,
			hour INT NULL,
			minute INT NULL,
			second INT NULL,
			display_num INT NULL,
			file_name TEXT
		)
	`)
	if err != nil {
		t.Fatalf("create screenshots table: %v", err)
	}

	return db
}
