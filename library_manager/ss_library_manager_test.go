package library_manager

import (
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"screenshot_server/Global"
	"screenshot_server/image_manipulation"
	"screenshot_server/utils"
	"strings"
	"sync"
	"testing"
)

type archiveTestEnv struct {
	cacheDir string
	imgDir   string
	db       *sql.DB
}

func setupArchiveTestEnv(t *testing.T) archiveTestEnv {
	t.Helper()
	root := t.TempDir()
	env := archiveTestEnv{
		cacheDir: filepath.Join(root, "cache"),
		imgDir:   filepath.Join(root, "img"),
	}
	if err := os.MkdirAll(env.cacheDir, os.ModePerm); err != nil {
		t.Fatalf("create cache dir: %v", err)
	}

	sig := 1
	Global.Globalsig_ss = &sig
	Global.Global_storage_errors = nil
	Global.Global_storage_errors_mutex = new(sync.Mutex)
	Global.Global_constant_config = &utils.Ss_constant_config{
		Cache_path:    env.cacheDir,
		Img_path:      env.imgDir,
		Database_path: filepath.Join(root, "test.db"),
	}

	env.db = Init_database()
	t.Cleanup(func() { env.db.Close() })
	Global.Global_database = env.db
	Global.Global_database_managebot = env.db
	schemaReady.Store(false)
	return env
}

func screenshotFileName(i int) string {
	return fmt.Sprintf("20261008_%02d%02d%02d_%d_16x16_%d.png", (i/3600)%24, (i/60)%60, i%60, i%2, 1000+i)
}

func writeScreenshotFixture(t *testing.T, dir string, fileName string, withMeta bool) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 140, B: 200, A: 255})
		}
	}
	path := filepath.Join(dir, fileName)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create fixture %s: %v", path, err)
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode fixture %s: %v", path, err)
	}
	file.Close()
	if withMeta {
		image_manipulation.Wirte_Meta_to_file(path, fileName, img)
	}
	return path
}

func countRows(t *testing.T, db *sql.DB, where string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM screenshots `+where, args...).Scan(&n); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

func TestInsertLibraryArchivesAcrossMultipleBatches(t *testing.T) {
	env := setupArchiveTestEnv(t)

	const total = 2*archiveBatchSize + 345
	files := make([]string, 0, total)
	for i := 0; i < total; i++ {
		// One file without EXIF metadata exercises the NULL-metadata insert.
		files = append(files, writeScreenshotFixture(t, env.cacheDir, screenshotFileName(i), i != 7))
	}

	// A stale row with the same file name but a legacy id must be overwritten.
	if err := EnsureScreenshotsMachineIDSchema(env.db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if _, err := env.db.Exec(`INSERT INTO screenshots (id, file_name, machine_id, hash) VALUES (?, ?, ?, ?)`,
		"legacy-id", screenshotFileName(3), defaultMachineID, "stale"); err != nil {
		t.Fatalf("insert stale row: %v", err)
	}

	if err := Insert_library(files); err != nil {
		t.Fatalf("Insert_library: %v", err)
	}

	if got := countRows(t, env.db, ""); got != total {
		t.Fatalf("row count: got %d want %d", got, total)
	}
	if got := countRows(t, env.db, `WHERE id = 'legacy-id' OR hash = 'stale'`); got != 0 {
		t.Fatalf("stale row was not overwritten")
	}
	if got := countRows(t, env.db, `WHERE display_num IS NULL OR hash_kind IS NULL OR year IS NULL`); got != 1 {
		t.Fatalf("rows missing metadata: got %d want 1 (the file without EXIF)", got)
	}

	var displayNum, year, second int
	var hashKind, id string
	if err := env.db.QueryRow(`SELECT id, display_num, hash_kind, year, second FROM screenshots WHERE file_name = ?`, screenshotFileName(1)).
		Scan(&id, &displayNum, &hashKind, &year, &second); err != nil {
		t.Fatalf("query archived row: %v", err)
	}
	if id != generateDefaultMachineScreenshotID(screenshotFileName(1)) || displayNum != 1 || hashKind == "" || year != 2026 || second != 1 {
		t.Fatalf("unexpected archived row: id=%s display=%d kind=%q year=%d second=%d", id, displayNum, hashKind, year, second)
	}

	cacheLeft, _ := os.ReadDir(env.cacheDir)
	if len(cacheLeft) != 0 {
		t.Fatalf("cache not emptied: %d files left", len(cacheLeft))
	}
	archived, _ := os.ReadDir(env.imgDir)
	if len(archived) != total {
		t.Fatalf("archived files: got %d want %d", len(archived), total)
	}
}

func TestInsertArchiveRecordsFallsBackWhenBatchFails(t *testing.T) {
	env := setupArchiveTestEnv(t)
	if err := EnsureScreenshotsMachineIDSchema(env.db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	bad := screenshotFileName(2)
	if _, err := env.db.Exec(fmt.Sprintf(`CREATE TRIGGER reject_bad BEFORE INSERT ON screenshots
		WHEN NEW.file_name = '%s' BEGIN SELECT RAISE(ABORT, 'rejected'); END`, bad)); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	// Stop signal makes the per-record retry give up after one failed attempt.
	*Global.Globalsig_ss = 0

	files := []string{}
	for i := 0; i < 5; i++ {
		files = append(files, writeScreenshotFixture(t, env.cacheDir, screenshotFileName(i), true))
	}
	insert_archive_records(prepareArchiveRecords(files, 2), env.db)

	if got := countRows(t, env.db, ""); got != 4 {
		t.Fatalf("row count after fallback: got %d want 4", got)
	}
	if got := countRows(t, env.db, `WHERE file_name = ?`, bad); got != 0 {
		t.Fatalf("rejected row was inserted")
	}
	if len(Global.Global_storage_errors) == 0 {
		t.Fatalf("expected the batch failure to be recorded as a storage error")
	}
}

func TestMemimgCheckingRobotInsertsOnlyMissingRows(t *testing.T) {
	env := setupArchiveTestEnv(t)
	if err := os.MkdirAll(env.imgDir, os.ModePerm); err != nil {
		t.Fatalf("create img dir: %v", err)
	}
	if err := EnsureScreenshotsMachineIDSchema(env.db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	const total = archiveBatchSize + 50
	for i := 0; i < total; i++ {
		writeScreenshotFixture(t, env.imgDir, screenshotFileName(i), true)
	}
	// Existing rows (one matched by id, one by file name) must be left untouched.
	if _, err := env.db.Exec(`INSERT INTO screenshots (id, file_name, machine_id, hash) VALUES (?, ?, ?, 'keep'), (?, ?, ?, 'keep')`,
		generateDefaultMachineScreenshotID(screenshotFileName(0)), screenshotFileName(0), defaultMachineID,
		"legacy-id", screenshotFileName(1), defaultMachineID); err != nil {
		t.Fatalf("insert existing rows: %v", err)
	}

	Memimg_checking_robot()

	if got := countRows(t, env.db, ""); got != total {
		t.Fatalf("row count: got %d want %d", got, total)
	}
	if got := countRows(t, env.db, `WHERE hash = 'keep'`); got != 2 {
		t.Fatalf("existing rows were modified: %d untouched, want 2", got)
	}
}

func TestEnsureSchemaCreatesFileNameIndex(t *testing.T) {
	env := setupArchiveTestEnv(t)
	if err := EnsureScreenshotsMachineIDSchema(env.db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	rows, err := env.db.Query(`EXPLAIN QUERY PLAN DELETE FROM screenshots WHERE id = ? OR (file_name = ? AND machine_id = ?)`, "a", "b", "c")
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close()
	plan := ""
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		plan += detail + "\n"
	}
	if !strings.Contains(plan, "idx_machine_file_name") || strings.Contains(plan, "SCAN screenshots") {
		t.Fatalf("dedup delete does not use the file name index:\n%s", plan)
	}
}
