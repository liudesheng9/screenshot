package tcp_api

import (
	"bufio"
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"screenshot_server/Global"
	"screenshot_server/utils"
)

func TestExecuteImgCopyStreamingSendsProgressAndDone(t *testing.T) {
	fileNames := make([]string, 30)
	for i := range fileNames {
		fileNames[i] = fmt.Sprintf("img_%02d.png", i)
	}

	imgPath := t.TempDir()
	createFixtureImages(t, imgPath, fileNames)

	db := createImageExportDB(t, fileNames)
	defer db.Close()

	restoreGlobals := installImageExportGlobals(db, imgPath)
	defer restoreGlobals()

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	safeConn := utils.Safe_connection{Conn: serverConn, Lock: &sync.Mutex{}}
	destPath := filepath.Join(t.TempDir(), "out")

	done := make(chan struct{})
	go func() {
		defer close(done)
		Execute_img(safeConn, "img copy 202501011000-1000 --stream "+destPath)
	}()

	reader := bufio.NewReader(clientConn)
	progressCount := 0
	doneLine := ""
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		line, err := reader.ReadString('\n')
		if err != nil {
			if opErr, ok := err.(net.Error); ok && opErr.Timeout() {
				continue
			}
			t.Fatalf("read streaming response: %v", err)
		}

		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PROGRESS ") {
			progressCount++
			continue
		}
		if strings.HasPrefix(line, "DONE ") {
			doneLine = line
			break
		}
		if strings.HasPrefix(line, "img error:") {
			t.Fatalf("unexpected img error response: %s", line)
		}
	}

	if progressCount == 0 {
		t.Fatalf("expected at least one PROGRESS line")
	}
	if doneLine == "" {
		t.Fatalf("expected DONE line")
	}
	for _, expected := range []string{"copied=30", "exist=30", "failed=0", "skipped=0"} {
		if !strings.Contains(doneLine, expected) {
			t.Fatalf("DONE line missing %q: %s", expected, doneLine)
		}
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for Execute_img completion")
	}
}

func TestExecuteImgCopyWithoutStreamSendsDoneOnly(t *testing.T) {
	fileNames := []string{"a.png", "b.png", "c.png"}
	imgPath := t.TempDir()
	createFixtureImages(t, imgPath, fileNames)

	db := createImageExportDB(t, fileNames)
	defer db.Close()

	restoreGlobals := installImageExportGlobals(db, imgPath)
	defer restoreGlobals()

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	safeConn := utils.Safe_connection{Conn: serverConn, Lock: &sync.Mutex{}}
	destPath := filepath.Join(t.TempDir(), "out")

	done := make(chan struct{})
	go func() {
		defer close(done)
		Execute_img(safeConn, "img copy 202501011000-1000 "+destPath)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 4096)
	n, err := clientConn.Read(buf)
	if err != nil {
		t.Fatalf("read non-stream response: %v", err)
	}

	output := strings.TrimSpace(string(buf[:n]))
	if !strings.HasPrefix(output, "DONE ") {
		t.Fatalf("expected DONE response, got %q", output)
	}
	if strings.Contains(output, "PROGRESS") {
		t.Fatalf("expected no PROGRESS for non-stream response, got %q", output)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for Execute_img completion")
	}
}

func createImageExportDB(t *testing.T, fileNames []string) *sql.DB {
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

	for _, fileName := range fileNames {
		_, err = db.Exec(
			`INSERT INTO screenshots(year, month, day, hour, minute, file_name) VALUES (?, ?, ?, ?, ?, ?)`,
			2025,
			1,
			1,
			10,
			0,
			fileName,
		)
		if err != nil {
			t.Fatalf("insert fixture %s: %v", fileName, err)
		}
	}

	return db
}

func installImageExportGlobals(db *sql.DB, imgPath string) func() {
	previousDB := Global.Global_database_net
	previousConfig := Global.Global_constant_config

	config := &utils.Ss_constant_config{}
	config.Init_ss_constant_config()
	config.Img_path = imgPath

	Global.Global_database_net = db
	Global.Global_constant_config = config

	return func() {
		Global.Global_database_net = previousDB
		Global.Global_constant_config = previousConfig
	}
}

func createFixtureImages(t *testing.T, root string, fileNames []string) {
	t.Helper()

	for _, fileName := range fileNames {
		path := filepath.Join(root, fileName)
		writeFixturePNG(t, path)
	}
}

func writeFixturePNG(t *testing.T, path string) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 120, G: 160, B: 220, A: 255})
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		t.Fatalf("create image directory for %s: %v", path, err)
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
