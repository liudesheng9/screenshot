package import_manager

import (
	"database/sql"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"screenshot_server/image_manipulation"
)

func TestImportDirectoryFullFlow(t *testing.T) {
	db := createImportManagerTestDB(t)
	defer db.Close()

	dir := t.TempDir()

	exifFileName := "20240115_143022_1_1920x1080_123.png"
	fallbackFileName := "20240116_010203_1.png"
	invalidFileName := "corrupt.png"

	writePNGWithEXIFFixture(t, filepath.Join(dir, exifFileName), exifFileName)
	writePNGFixture(t, filepath.Join(dir, fallbackFileName))
	if err := os.WriteFile(filepath.Join(dir, invalidFileName), []byte("not-a-real-png"), 0644); err != nil {
		t.Fatalf("write invalid png fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("ignored"), 0644); err != nil {
		t.Fatalf("write non-png fixture: %v", err)
	}

	progressUpdates := 0
	result, err := ImportDirectory(ImportConfig{
		DB:          db,
		Directory:   dir,
		Remap:       map[int]int{1: 3},
		BatchSize:   2,
		WorkerCount: 2,
		ProgressCallback: func(progress ImportProgress) {
			progressUpdates++
		},
	})
	if err != nil {
		t.Fatalf("ImportDirectory returned error: %v", err)
	}

	if result.Total != 3 {
		t.Fatalf("expected total=3 png files, got %d", result.Total)
	}
	if result.Processed != 3 {
		t.Fatalf("expected processed=3, got %d", result.Processed)
	}
	if result.Inserted != 2 {
		t.Fatalf("expected inserted=2, got %d", result.Inserted)
	}
	if result.Failed != 1 {
		t.Fatalf("expected failed=1, got %d", result.Failed)
	}
	if result.Skipped != 0 {
		t.Fatalf("expected skipped=0, got %d", result.Skipped)
	}
	if progressUpdates == 0 {
		t.Fatalf("expected progress callback to be called")
	}

	assertScreenshotRowCount(t, db, 2)

	var fallbackDisplay int
	var fallbackHashKind sql.NullString
	var fallbackMachineID string
	err = db.QueryRow(
		`SELECT display_num, hash_kind, machine_id FROM screenshots WHERE file_name = ?`,
		fallbackFileName,
	).Scan(&fallbackDisplay, &fallbackHashKind, &fallbackMachineID)
	if err != nil {
		t.Fatalf("query fallback record: %v", err)
	}
	if fallbackDisplay != 3 {
		t.Fatalf("expected remapped display_num=3 for fallback record, got %d", fallbackDisplay)
	}
	if fallbackHashKind.Valid {
		t.Fatalf("expected fallback hash_kind to be NULL, got %q", fallbackHashKind.String)
	}
	if fallbackMachineID != DefaultMachineID {
		t.Fatalf("expected fallback machine_id=%q, got %q", DefaultMachineID, fallbackMachineID)
	}

	var exifHashKind sql.NullString
	err = db.QueryRow(
		`SELECT hash_kind FROM screenshots WHERE file_name = ?`,
		exifFileName,
	).Scan(&exifHashKind)
	if err != nil {
		t.Fatalf("query exif record: %v", err)
	}
	if !exifHashKind.Valid || exifHashKind.String == "" {
		t.Fatalf("expected EXIF-backed record to include hash_kind")
	}
}

func TestImportDirectoryEmptyDirectory(t *testing.T) {
	db := createImportManagerTestDB(t)
	defer db.Close()

	result, err := ImportDirectory(ImportConfig{
		DB:        db,
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("ImportDirectory returned error for empty directory: %v", err)
	}
	if result.Total != 0 {
		t.Fatalf("expected total=0 for empty directory, got %d", result.Total)
	}
	if result.Processed != 0 {
		t.Fatalf("expected processed=0 for empty directory, got %d", result.Processed)
	}
	if result.Inserted != 0 || result.Failed != 0 || result.Skipped != 0 {
		t.Fatalf("expected zeroed result for empty directory, got %+v", result)
	}
}

func TestImportDirectoryDedupSkipByID(t *testing.T) {
	db := createImportManagerTestDB(t)
	defer db.Close()

	dir := t.TempDir()
	fileName := "20240101_000000_1.png"
	writePNGFixture(t, filepath.Join(dir, fileName))

	fileID := GenerateScreenshotID(DefaultMachineID, fileName)
	_, err := db.Exec(
		`INSERT INTO screenshots (id, file_name, year, month, day, hour, minute, second, display_num, machine_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fileID,
		fileName,
		2024,
		1,
		1,
		0,
		0,
		0,
		1,
		DefaultMachineID,
	)
	if err != nil {
		t.Fatalf("insert existing fixture: %v", err)
	}

	result, err := ImportDirectory(ImportConfig{DB: db, Directory: dir})
	if err != nil {
		t.Fatalf("ImportDirectory returned error: %v", err)
	}
	if result.Skipped != 1 {
		t.Fatalf("expected skipped=1, got %d", result.Skipped)
	}
	if result.Inserted != 0 {
		t.Fatalf("expected inserted=0, got %d", result.Inserted)
	}
	if result.Updated != 0 {
		t.Fatalf("expected updated=0, got %d", result.Updated)
	}
	if result.Failed != 0 {
		t.Fatalf("expected failed=0, got %d", result.Failed)
	}
}

func TestImportDirectoryDedupUpdateByFilename(t *testing.T) {
	db := createImportManagerTestDB(t)
	defer db.Close()

	dir := t.TempDir()
	fileName := "20240131_235959_5.png"
	writePNGFixture(t, filepath.Join(dir, fileName))

	_, err := db.Exec(
		`INSERT INTO screenshots (id, file_name, year, month, day, hour, minute, second, display_num, machine_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"legacy-id",
		fileName,
		2000,
		1,
		1,
		0,
		0,
		0,
		0,
		DefaultMachineID,
	)
	if err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}

	result, err := ImportDirectory(ImportConfig{DB: db, Directory: dir})
	if err != nil {
		t.Fatalf("ImportDirectory returned error: %v", err)
	}
	if result.Updated != 1 {
		t.Fatalf("expected updated=1, got %d", result.Updated)
	}

	var id string
	var year, month, day, hour, minute, second, display int
	err = db.QueryRow(
		`SELECT id, year, month, day, hour, minute, second, display_num FROM screenshots WHERE file_name = ?`,
		fileName,
	).Scan(&id, &year, &month, &day, &hour, &minute, &second, &display)
	if err != nil {
		t.Fatalf("query updated row: %v", err)
	}

	if id != GenerateScreenshotID(DefaultMachineID, fileName) {
		t.Fatalf("expected updated id %s, got %s", GenerateScreenshotID(DefaultMachineID, fileName), id)
	}
	if year != 2024 || month != 1 || day != 31 || hour != 23 || minute != 59 || second != 59 || display != 5 {
		t.Fatalf("unexpected parsed timestamp/display in updated row: %d-%d-%d %d:%d:%d d=%d", year, month, day, hour, minute, second, display)
	}
}

func TestImportDirectoryBatchFallbackContinuesOnFileError(t *testing.T) {
	db := createImportManagerTestDB(t)
	defer db.Close()

	dir := t.TempDir()
	goodFile := "20240201_120000_1.png"
	failingFile := "20240201_120001_1.png"

	writePNGFixture(t, filepath.Join(dir, goodFile))
	writePNGFixture(t, filepath.Join(dir, failingFile))

	_, err := db.Exec(`
		CREATE TRIGGER fail_import_insert
		BEFORE INSERT ON screenshots
		WHEN NEW.file_name = '20240201_120001_1.png'
		BEGIN
			SELECT RAISE(FAIL, 'forced insert failure');
		END;
	`)
	if err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	result, err := ImportDirectory(ImportConfig{
		DB:        db,
		Directory: dir,
		BatchSize: 100,
	})
	if err != nil {
		t.Fatalf("ImportDirectory returned error: %v", err)
	}

	if result.Inserted != 1 {
		t.Fatalf("expected one successful insert after fallback, got %d", result.Inserted)
	}
	if result.Failed != 1 {
		t.Fatalf("expected one failed record after fallback, got %d", result.Failed)
	}
	if result.BatchFallbackUsed != 1 {
		t.Fatalf("expected fallback to be used once, got %d", result.BatchFallbackUsed)
	}

	assertScreenshotRowCount(t, db, 1)
}

func TestImportDirectoryEdgeCases(t *testing.T) {
	db := createImportManagerTestDB(t)
	defer db.Close()

	dir := t.TempDir()
	validCorruptFile := "20240117_111111_2.png"
	invalidFile := "screenshot.png"

	if err := os.WriteFile(filepath.Join(dir, validCorruptFile), []byte("corrupt-but-parseable-name"), 0644); err != nil {
		t.Fatalf("write valid-name corrupt file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, invalidFile), []byte("corrupt-and-unparseable"), 0644); err != nil {
		t.Fatalf("write invalid-name corrupt file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte("not a png"), 0644); err != nil {
		t.Fatalf("write non-png fixture: %v", err)
	}

	result, err := ImportDirectory(ImportConfig{DB: db, Directory: dir})
	if err != nil {
		t.Fatalf("ImportDirectory returned error: %v", err)
	}

	if result.Total != 2 {
		t.Fatalf("expected total png files=2, got %d", result.Total)
	}
	if result.Inserted != 1 {
		t.Fatalf("expected inserted=1, got %d", result.Inserted)
	}
	if result.Failed != 1 {
		t.Fatalf("expected failed=1, got %d", result.Failed)
	}

	assertScreenshotRowCount(t, db, 1)
}

func TestImportDirectoryWithMachineIDProducesMachineScopedID(t *testing.T) {
	db := createImportManagerTestDB(t)
	defer db.Close()

	dir := t.TempDir()
	fileName := "20240210_101010_1.png"
	writePNGFixture(t, filepath.Join(dir, fileName))

	result, err := ImportDirectory(ImportConfig{
		DB:        db,
		Directory: dir,
		MachineID: "laptop1",
	})
	if err != nil {
		t.Fatalf("ImportDirectory returned error: %v", err)
	}
	if result.Inserted != 1 {
		t.Fatalf("expected inserted=1, got %d", result.Inserted)
	}

	var id string
	var machineID string
	err = db.QueryRow(`SELECT id, machine_id FROM screenshots WHERE file_name = ?`, fileName).Scan(&id, &machineID)
	if err != nil {
		t.Fatalf("query imported row: %v", err)
	}
	if machineID != "laptop1" {
		t.Fatalf("expected machine_id=laptop1, got %q", machineID)
	}
	wantID := GenerateScreenshotID("laptop1", fileName)
	if id != wantID {
		t.Fatalf("unexpected id: got=%s want=%s", id, wantID)
	}
}

func TestImportDirectorySameFilenameDifferentMachinesCreateDistinctRecords(t *testing.T) {
	db := createImportManagerTestDB(t)
	defer db.Close()

	fileName := "20240211_111111_2.png"
	dirA := t.TempDir()
	dirB := t.TempDir()
	writePNGFixture(t, filepath.Join(dirA, fileName))
	writePNGFixture(t, filepath.Join(dirB, fileName))

	firstResult, err := ImportDirectory(ImportConfig{DB: db, Directory: dirA, MachineID: "laptop1"})
	if err != nil {
		t.Fatalf("first ImportDirectory returned error: %v", err)
	}
	if firstResult.Inserted != 1 {
		t.Fatalf("expected first insert count=1, got %d", firstResult.Inserted)
	}

	secondResult, err := ImportDirectory(ImportConfig{DB: db, Directory: dirB, MachineID: "desktop1"})
	if err != nil {
		t.Fatalf("second ImportDirectory returned error: %v", err)
	}
	if secondResult.Inserted != 1 {
		t.Fatalf("expected second insert count=1, got %d", secondResult.Inserted)
	}

	var count int
	var distinctIDs int
	err = db.QueryRow(`SELECT COUNT(*), COUNT(DISTINCT id) FROM screenshots WHERE file_name = ?`, fileName).Scan(&count, &distinctIDs)
	if err != nil {
		t.Fatalf("count machine-scoped rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected two rows for same filename across machines, got %d", count)
	}
	if distinctIDs != 2 {
		t.Fatalf("expected two distinct ids for same filename across machines, got %d", distinctIDs)
	}
}

func TestImportDirectorySameFilenameSameMachineSkipsDuplicate(t *testing.T) {
	db := createImportManagerTestDB(t)
	defer db.Close()

	fileName := "20240212_121212_3.png"
	dir := t.TempDir()
	writePNGFixture(t, filepath.Join(dir, fileName))

	firstResult, err := ImportDirectory(ImportConfig{DB: db, Directory: dir, MachineID: "laptop1"})
	if err != nil {
		t.Fatalf("first ImportDirectory returned error: %v", err)
	}
	if firstResult.Inserted != 1 {
		t.Fatalf("expected first insert count=1, got %d", firstResult.Inserted)
	}

	secondResult, err := ImportDirectory(ImportConfig{DB: db, Directory: dir, MachineID: "laptop1"})
	if err != nil {
		t.Fatalf("second ImportDirectory returned error: %v", err)
	}
	if secondResult.Skipped != 1 {
		t.Fatalf("expected duplicate import to be skipped, got skipped=%d", secondResult.Skipped)
	}

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM screenshots WHERE file_name = ? AND machine_id = ?`, fileName, "laptop1").Scan(&count)
	if err != nil {
		t.Fatalf("count same-machine rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one row for same machine+filename, got %d", count)
	}
}

func TestImportDirectoryWithoutMachineUsesDefaultMachine(t *testing.T) {
	db := createImportManagerTestDB(t)
	defer db.Close()

	dir := t.TempDir()
	fileName := "20240213_131313_4.png"
	writePNGFixture(t, filepath.Join(dir, fileName))

	result, err := ImportDirectory(ImportConfig{DB: db, Directory: dir})
	if err != nil {
		t.Fatalf("ImportDirectory returned error: %v", err)
	}
	if result.Inserted != 1 {
		t.Fatalf("expected inserted=1, got %d", result.Inserted)
	}

	var id string
	var machineID string
	err = db.QueryRow(`SELECT id, machine_id FROM screenshots WHERE file_name = ?`, fileName).Scan(&id, &machineID)
	if err != nil {
		t.Fatalf("query imported row: %v", err)
	}
	if machineID != DefaultMachineID {
		t.Fatalf("expected default machine_id=%q, got %q", DefaultMachineID, machineID)
	}
	if id != GenerateScreenshotID(DefaultMachineID, fileName) {
		t.Fatalf("unexpected id for default machine: got=%s want=%s", id, GenerateScreenshotID(DefaultMachineID, fileName))
	}
}

func assertScreenshotRowCount(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var got int
	err := db.QueryRow(`SELECT COUNT(*) FROM screenshots`).Scan(&got)
	if err != nil {
		t.Fatalf("count screenshots rows: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected row count: got=%d want=%d", got, want)
	}
}

func writePNGWithEXIFFixture(t *testing.T, path string, fileName string) {
	t.Helper()
	img := newFixtureImage()
	writePNGImage(t, path, img)
	image_manipulation.Wirte_Meta_to_file(path, fileName, img)
}

func writePNGFixture(t *testing.T, path string) {
	t.Helper()
	writePNGImage(t, path, newFixtureImage())
}

func writePNGImage(t *testing.T, path string, img *image.RGBA) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create fixture image %s: %v", path, err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode fixture image %s: %v", path, err)
	}
}

func newFixtureImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 140, B: 200, A: 255})
		}
	}
	return img
}
