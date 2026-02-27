package image_export

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
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
		ProgressConfig{EveryImages: 1000, EveryInterval: time.Millisecond},
	)

	if copied != taskTotal || failed != 0 {
		t.Fatalf("unexpected copy result copied=%d failed=%d", copied, failed)
	}
	for _, task := range tasks {
		if _, err := os.Stat(task.targetPath); err != nil {
			t.Fatalf("expected transformed output %s: %v", task.targetPath, err)
		}
	}
	if len(progress) < 2 {
		t.Fatalf("expected at least 2 progress updates, got %d", len(progress))
	}

	hasActiveWorkerTask := false
	for len(progress) > 0 {
		update := <-progress
		if update.Target != taskTotal {
			t.Fatalf("unexpected target count: got=%d want=%d", update.Target, taskTotal)
		}
		for _, workerTask := range update.WorkerTasks {
			if workerTask != nil {
				hasActiveWorkerTask = true
				break
			}
		}
		if hasActiveWorkerTask {
			break
		}
	}
	if !hasActiveWorkerTask {
		t.Fatalf("expected at least one progress update with active worker task")
	}
}

func TestRunIOWorkerBuffersImage(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "source.png")
	writePNGFixture(t, srcPath)
	targetPath := filepath.Join(t.TempDir(), "target.jpg")

	taskQueue := make(chan imageTransformTask, 1)
	bufferedImageQueue := make(chan *BufferedImage, 1)
	resultQueue := make(chan imageTransformResult, 1)
	stats := NewPipelineWorkerStats(1, 1)
	defer stats.Close()

	taskQueue <- imageTransformTask{sourcePath: srcPath, targetPath: targetPath}
	close(taskQueue)

	runIOWorker(0, taskQueue, bufferedImageQueue, resultQueue, stats)

	if len(resultQueue) != 0 {
		result := <-resultQueue
		t.Fatalf("expected no I/O error, got: %v", result.err)
	}
	if len(bufferedImageQueue) != 1 {
		t.Fatalf("expected one buffered image, got %d", len(bufferedImageQueue))
	}

	bufferedImage := <-bufferedImageQueue
	if bufferedImage.SourcePath != srcPath {
		t.Fatalf("unexpected source path: %s", bufferedImage.SourcePath)
	}
	if bufferedImage.TargetPath != targetPath {
		t.Fatalf("unexpected target path: %s", bufferedImage.TargetPath)
	}
	if len(bufferedImage.Data) == 0 {
		t.Fatalf("expected buffered image data to be populated")
	}

	waitForWorkerCount(t, stats, ioWorkerInternalID(0), 1)
}

func TestRunIOWorkerReportsReadError(t *testing.T) {
	taskQueue := make(chan imageTransformTask, 1)
	bufferedImageQueue := make(chan *BufferedImage, 1)
	resultQueue := make(chan imageTransformResult, 1)

	taskQueue <- imageTransformTask{
		sourcePath: filepath.Join(t.TempDir(), "missing.png"),
		targetPath: filepath.Join(t.TempDir(), "target.jpg"),
	}
	close(taskQueue)

	runIOWorker(0, taskQueue, bufferedImageQueue, resultQueue, nil)

	if len(bufferedImageQueue) != 0 {
		t.Fatalf("expected no buffered image on read error")
	}
	if len(resultQueue) != 1 {
		t.Fatalf("expected one error result, got %d", len(resultQueue))
	}
	result := <-resultQueue
	if result.err == nil {
		t.Fatalf("expected read error result")
	}
}

func TestRunIOWorkerBlocksWhenBufferedQueueIsFull(t *testing.T) {
	srcDir := t.TempDir()
	taskQueue := make(chan imageTransformTask, 2)
	for i := 0; i < 2; i++ {
		srcPath := filepath.Join(srcDir, fmt.Sprintf("src_%02d.png", i))
		writePNGFixture(t, srcPath)
		taskQueue <- imageTransformTask{
			sourcePath: srcPath,
			targetPath: filepath.Join(t.TempDir(), fmt.Sprintf("dst_%02d.jpg", i)),
		}
	}
	close(taskQueue)

	bufferedImageQueue := make(chan *BufferedImage, 1)
	resultQueue := make(chan imageTransformResult, 2)

	done := make(chan struct{})
	go func() {
		defer close(done)
		runIOWorker(0, taskQueue, bufferedImageQueue, resultQueue, nil)
	}()

	time.Sleep(100 * time.Millisecond)
	if len(bufferedImageQueue) != 1 {
		t.Fatalf("expected buffer queue to be full with one item, got %d", len(bufferedImageQueue))
	}
	select {
	case <-done:
		t.Fatalf("expected I/O worker to block while buffer queue is full")
	default:
	}

	<-bufferedImageQueue
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for I/O worker to finish after draining queue")
	}
}

func TestProcessImageTransformQueueIncludesPipelineWorkerRefs(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	taskTotal := 12
	tasks := make([]imageTransformTask, 0, taskTotal)
	for i := 0; i < taskTotal; i++ {
		srcPath := filepath.Join(srcDir, fmt.Sprintf("src_%02d.png", i))
		targetPath := filepath.Join(destDir, fmt.Sprintf("dst_%02d.jpg", i))
		writePNGFixture(t, srcPath)
		tasks = append(tasks, imageTransformTask{sourcePath: srcPath, targetPath: targetPath})
	}

	progress := make(chan ProgressUpdate, 256)
	copied, failed := processImageTransformQueue(
		tasks,
		4,
		defaultJPEGQuality,
		progress,
		ProgressConfig{EveryImages: 1, EveryInterval: time.Millisecond},
	)

	if copied != taskTotal || failed != 0 {
		t.Fatalf("unexpected copy result copied=%d failed=%d", copied, failed)
	}

	sawIOWorker := false
	sawProcessingWorker := false
	for len(progress) > 0 {
		update := <-progress
		for _, workerRef := range update.WorkerRefs {
			switch workerRef.WorkerType {
			case WorkerTypeIO:
				sawIOWorker = true
			case WorkerTypePROC:
				sawProcessingWorker = true
			}
		}
	}

	if !sawIOWorker {
		t.Fatalf("expected progress updates to include IO worker refs")
	}
	if !sawProcessingWorker {
		t.Fatalf("expected progress updates to include processing worker refs")
	}
}

func TestWorkerStatsImplementsStageReporter(t *testing.T) {
	stats := NewWorkerStats(1)
	defer stats.Close()

	var reporter StageReporter = stats
	reporter.ReportStage(0, "img_01.png", StageRead)

	tasks := stats.GetWorkerTasks()
	task := tasks[0]
	if task == nil {
		t.Fatalf("expected worker task to be tracked")
	}
	if task.Filename != "img_01.png" {
		t.Fatalf("unexpected filename: %s", task.Filename)
	}
	if task.Stage != StageRead {
		t.Fatalf("unexpected stage: %s", task.Stage)
	}
	if task.StartTime.IsZero() {
		t.Fatalf("expected non-zero start time")
	}

	startedAt := task.StartTime
	reporter.ReportStage(0, "img_01.png", StageDecode)
	tasks = stats.GetWorkerTasks()
	task = tasks[0]
	if task == nil {
		t.Fatalf("expected worker task to remain tracked")
	}
	if task.Stage != StageDecode {
		t.Fatalf("unexpected stage transition: %s", task.Stage)
	}
	if !task.StartTime.Equal(startedAt) {
		t.Fatalf("expected start time to remain constant across stage transitions")
	}

	reporter.ClearWorkerTask(0)
	tasks = stats.GetWorkerTasks()
	if task := tasks[0]; task != nil {
		t.Fatalf("expected worker task to be cleared")
	}
}

func TestWorkerStatsReportStageAndClearThreadSafety(t *testing.T) {
	stats := NewWorkerStats(4)
	defer stats.Close()

	var wg sync.WaitGroup
	for workerID := 0; workerID < 4; workerID++ {
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id, idx int) {
				defer wg.Done()
				fileName := fmt.Sprintf("img_%d_%d.png", id, idx)
				stats.ReportStage(id, fileName, StageRead)
				stats.ReportStage(id, fileName, StageDecode)
				if idx%3 == 0 {
					stats.ClearWorkerTask(id)
				}
			}(workerID, i)
		}
	}
	wg.Wait()

	tasks := stats.GetWorkerTasks()
	if len(tasks) != 4 {
		t.Fatalf("expected task map to include all workers, got %d", len(tasks))
	}
}

func TestConvertImageToJPEGHandlesNilReporter(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "source.png")
	writePNGFixture(t, srcPath)
	destPath := filepath.Join(t.TempDir(), "target.jpg")
	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read fixture source image: %v", err)
	}

	err = convertImageToJPEG(&BufferedImage{
		SourcePath: srcPath,
		TargetPath: destPath,
		Data:       data,
	}, defaultJPEGQuality, processingWorkerInternalID(0), nil)
	if err != nil {
		t.Fatalf("convert image with nil reporter: %v", err)
	}

	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("expected jpeg file to exist: %v", err)
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

func BenchmarkProcessImageTransformQueue(b *testing.B) {
	srcDir := b.TempDir()
	destRoot := b.TempDir()

	taskTotal := 60
	sourcePaths := make([]string, 0, taskTotal)
	for i := 0; i < taskTotal; i++ {
		srcPath := filepath.Join(srcDir, fmt.Sprintf("src_%02d.png", i))
		writeBenchmarkPNGFixture(b, srcPath)
		sourcePaths = append(sourcePaths, srcPath)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		destDir := filepath.Join(destRoot, fmt.Sprintf("run_%d", i))
		if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
			b.Fatalf("create benchmark destination directory: %v", err)
		}

		tasks := make([]imageTransformTask, 0, len(sourcePaths))
		for _, srcPath := range sourcePaths {
			targetName, err := toJPEGFileName(filepath.Base(srcPath))
			if err != nil {
				b.Fatalf("build benchmark target name: %v", err)
			}
			tasks = append(tasks, imageTransformTask{
				sourcePath: srcPath,
				targetPath: filepath.Join(destDir, targetName),
			})
		}

		copied, failed := processImageTransformQueue(
			tasks,
			defaultProcessingWorkers,
			defaultJPEGQuality,
			nil,
			defaultProgressConfig,
		)
		if copied != len(tasks) || failed != 0 {
			b.Fatalf("unexpected benchmark result copied=%d failed=%d", copied, failed)
		}

		if err := clearDirectoryContents(destDir); err != nil {
			b.Fatalf("clear benchmark destination directory: %v", err)
		}
	}
}

func writeBenchmarkPNGFixture(b *testing.B, path string) {
	b.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	file, err := os.Create(path)
	if err != nil {
		b.Fatalf("create fixture image %s: %v", path, err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		b.Fatalf("encode fixture image %s: %v", path, err)
	}
}
