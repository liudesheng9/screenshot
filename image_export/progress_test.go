package image_export

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkerStatsSingleWorkerReports(t *testing.T) {
	stats := NewWorkerStats(1)
	defer stats.Close()

	for i := 0; i < 3; i++ {
		stats.Report(0)
	}

	waitForWorkerCount(t, stats, 0, 3)
}

func TestWorkerStatsMultipleWorkersHaveDistinctIDs(t *testing.T) {
	stats := NewWorkerStats(10)
	defer stats.Close()

	for workerID := 0; workerID < 10; workerID++ {
		stats.Report(workerID)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		counts := stats.GetWorkerCounts()
		allMatched := true
		for workerID := 0; workerID < 10; workerID++ {
			if counts[workerID] != 1 {
				allMatched = false
				break
			}
		}
		if allMatched {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	counts := stats.GetWorkerCounts()
	t.Fatalf("expected one event per worker, got: %#v", counts)
}

func TestSlidingWindowAgesOutOldEvents(t *testing.T) {
	window := NewSlidingWindow(8, 30*time.Second)
	now := time.Now()

	window.Add(now.Add(-40 * time.Second))
	window.Add(now.Add(-3 * time.Second))

	if got := window.Count(); got != 1 {
		t.Fatalf("expected 1 event inside 30s window, got %d", got)
	}
}

func TestProcessImageTransformQueueSendsIntermediateProgress(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	taskTotal := 30
	tasks := make([]imageTransformTask, 0, taskTotal)
	for i := 0; i < taskTotal; i++ {
		srcPath := filepath.Join(srcDir, fmt.Sprintf("src_%02d.png", i))
		targetPath := filepath.Join(destDir, fmt.Sprintf("dst_%02d.jpg", i))
		writePNGFixture(t, srcPath)
		tasks = append(tasks, imageTransformTask{sourcePath: srcPath, targetPath: targetPath})
	}

	progress := make(chan ProgressUpdate, 128)
	copied, failed := processImageTransformQueue(
		tasks,
		1,
		defaultJPEGQuality,
		progress,
		ProgressConfig{EveryImages: 5, EveryInterval: time.Hour},
	)

	if copied != taskTotal || failed != 0 {
		t.Fatalf("unexpected copy result copied=%d failed=%d", copied, failed)
	}
	if len(progress) < 2 {
		t.Fatalf("expected at least 2 progress updates, got %d", len(progress))
	}
}

func waitForWorkerCount(t *testing.T, stats *WorkerStats, workerID, want int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		counts := stats.GetWorkerCounts()
		if counts[workerID] == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	counts := stats.GetWorkerCounts()
	t.Fatalf("worker %d count mismatch: want=%d got=%d full=%#v", workerID, want, counts[workerID], counts)
}

func writePNGFixture(t *testing.T, path string) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
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
