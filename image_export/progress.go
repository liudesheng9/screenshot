package image_export

import (
	"fmt"
	"sync"
	"time"
)

const (
	defaultSlidingWindowSize  = 1024
	defaultSlidingWindowRange = 30 * time.Second
	progressEventBufferSize   = 100
	processingWorkerIDOffset  = 1000
)

type WorkerType string

const (
	WorkerTypeDefault WorkerType = ""
	WorkerTypeIO      WorkerType = "IO"
	WorkerTypePROC    WorkerType = "PROC"
)

type Stage string

const (
	StageReading Stage = "reading"
	StageRead    Stage = "read"
	StageDecode  Stage = "decode"
	StageEncode  Stage = "encode"
	StageWrite   Stage = "write"
	StageSync    Stage = "sync"
)

type WorkerRef struct {
	WorkerType WorkerType
	WorkerID   int
}

func (wr WorkerRef) Label() string {
	switch wr.WorkerType {
	case WorkerTypeIO:
		return fmt.Sprintf("IO-W%d", wr.WorkerID)
	case WorkerTypePROC:
		return fmt.Sprintf("PROC-W%d", wr.WorkerID)
	default:
		return fmt.Sprintf("W%d", wr.WorkerID)
	}
}

type WorkerTask struct {
	WorkerType WorkerType
	Filename   string
	Stage      Stage
	StartTime  time.Time
}

type StageReporter interface {
	ReportStage(workerID int, filename string, stage Stage)
	ClearWorkerTask(workerID int)
}

type ProgressUpdate struct {
	WorkerCounts map[int]int
	WorkerTasks  map[int]*WorkerTask
	WorkerRefs   map[int]WorkerRef
	Total        int
	Target       int
	Timestamp    time.Time
}

type SlidingWindow struct {
	mu     sync.Mutex
	events []time.Time
	head   int
	count  int
	window time.Duration
}

func NewSlidingWindow(capacity int, window time.Duration) *SlidingWindow {
	if capacity < 1 {
		capacity = defaultSlidingWindowSize
	}
	if window <= 0 {
		window = defaultSlidingWindowRange
	}
	return &SlidingWindow{
		events: make([]time.Time, capacity),
		window: window,
	}
}

func (sw *SlidingWindow) Add(eventTime time.Time) {
	if sw == nil {
		return
	}
	if eventTime.IsZero() {
		eventTime = time.Now()
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.count < len(sw.events) {
		writeIdx := (sw.head + sw.count) % len(sw.events)
		sw.events[writeIdx] = eventTime
		sw.count++
	} else {
		sw.events[sw.head] = eventTime
		sw.head = (sw.head + 1) % len(sw.events)
	}

	sw.trimLocked(eventTime)
}

func (sw *SlidingWindow) Count() int {
	if sw == nil {
		return 0
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.trimLocked(time.Now())
	return sw.count
}

func (sw *SlidingWindow) trimLocked(now time.Time) {
	cutoff := now.Add(-sw.window)
	for sw.count > 0 {
		oldest := sw.events[sw.head]
		if !oldest.Before(cutoff) {
			break
		}
		sw.head = (sw.head + 1) % len(sw.events)
		sw.count--
	}
}

type WorkerStats struct {
	mu         sync.RWMutex
	windows    map[int]*SlidingWindow
	tasks      map[int]*WorkerTask
	workerRefs map[int]WorkerRef
	aggregator *ProgressAggregator
}

var _ StageReporter = (*WorkerStats)(nil)

func NewWorkerStats(workerCount int) *WorkerStats {
	if workerCount < 1 {
		workerCount = 1
	}

	stats := &WorkerStats{
		windows:    make(map[int]*SlidingWindow, workerCount),
		tasks:      make(map[int]*WorkerTask, workerCount),
		workerRefs: make(map[int]WorkerRef, workerCount),
	}
	for workerID := 0; workerID < workerCount; workerID++ {
		stats.windows[workerID] = NewSlidingWindow(defaultSlidingWindowSize, defaultSlidingWindowRange)
		stats.workerRefs[workerID] = WorkerRef{WorkerType: WorkerTypeDefault, WorkerID: workerID}
	}
	stats.aggregator = NewProgressAggregator(stats)
	return stats
}

func NewPipelineWorkerStats(ioWorkerCount, procWorkerCount int) *WorkerStats {
	if ioWorkerCount < 1 {
		ioWorkerCount = 1
	}
	if procWorkerCount < 1 {
		procWorkerCount = 1
	}

	totalWorkers := ioWorkerCount + procWorkerCount
	stats := &WorkerStats{
		windows:    make(map[int]*SlidingWindow, totalWorkers),
		tasks:      make(map[int]*WorkerTask, totalWorkers),
		workerRefs: make(map[int]WorkerRef, totalWorkers),
	}
	for workerID := 0; workerID < ioWorkerCount; workerID++ {
		internalID := ioWorkerInternalID(workerID)
		stats.windows[internalID] = NewSlidingWindow(defaultSlidingWindowSize, defaultSlidingWindowRange)
		stats.workerRefs[internalID] = WorkerRef{WorkerType: WorkerTypeIO, WorkerID: workerID}
	}
	for workerID := 0; workerID < procWorkerCount; workerID++ {
		internalID := processingWorkerInternalID(workerID)
		stats.windows[internalID] = NewSlidingWindow(defaultSlidingWindowSize, defaultSlidingWindowRange)
		stats.workerRefs[internalID] = WorkerRef{WorkerType: WorkerTypePROC, WorkerID: workerID}
	}
	stats.aggregator = NewProgressAggregator(stats)
	return stats
}

func ioWorkerInternalID(workerID int) int {
	return workerID
}

func processingWorkerInternalID(workerID int) int {
	return processingWorkerIDOffset + workerID
}

func inferWorkerRef(workerID int) WorkerRef {
	if workerID >= processingWorkerIDOffset {
		return WorkerRef{WorkerType: WorkerTypePROC, WorkerID: workerID - processingWorkerIDOffset}
	}
	return WorkerRef{WorkerType: WorkerTypeDefault, WorkerID: workerID}
}

func InferWorkerRef(workerID int) WorkerRef {
	return inferWorkerRef(workerID)
}

func (ws *WorkerStats) workerRef(workerID int) WorkerRef {
	if ws == nil {
		return inferWorkerRef(workerID)
	}

	ws.mu.RLock()
	ref, ok := ws.workerRefs[workerID]
	ws.mu.RUnlock()
	if ok {
		return ref
	}

	ref = inferWorkerRef(workerID)
	ws.mu.Lock()
	if _, exists := ws.workerRefs[workerID]; !exists {
		ws.workerRefs[workerID] = ref
	}
	ws.mu.Unlock()
	return ref
}

func (ws *WorkerStats) Report(workerID int) {
	if ws == nil || ws.aggregator == nil {
		return
	}
	ws.aggregator.Report(workerID)
}

func (ws *WorkerStats) GetWorkerCounts() map[int]int {
	if ws == nil {
		return map[int]int{}
	}

	ws.mu.RLock()
	defer ws.mu.RUnlock()

	counts := make(map[int]int, len(ws.windows))
	for workerID, window := range ws.windows {
		counts[workerID] = window.Count()
	}
	return counts
}

func (ws *WorkerStats) GetWorkerRefs() map[int]WorkerRef {
	if ws == nil {
		return map[int]WorkerRef{}
	}

	ws.mu.RLock()
	defer ws.mu.RUnlock()

	refs := make(map[int]WorkerRef, len(ws.workerRefs))
	for workerID, ref := range ws.workerRefs {
		refs[workerID] = ref
	}
	return refs
}

func (ws *WorkerStats) ReportStage(workerID int, filename string, stage Stage) {
	if ws == nil {
		return
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	ref, ok := ws.workerRefs[workerID]
	if !ok {
		ref = inferWorkerRef(workerID)
		ws.workerRefs[workerID] = ref
	}

	startTime := time.Now()
	if existingTask, ok := ws.tasks[workerID]; ok && existingTask != nil && existingTask.Filename == filename {
		if !existingTask.StartTime.IsZero() {
			startTime = existingTask.StartTime
		}
	}

	ws.tasks[workerID] = &WorkerTask{
		WorkerType: ref.WorkerType,
		Filename:   filename,
		Stage:      stage,
		StartTime:  startTime,
	}
}

func (ws *WorkerStats) ClearWorkerTask(workerID int) {
	if ws == nil {
		return
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()
	delete(ws.tasks, workerID)
}

func (ws *WorkerStats) GetWorkerTasks() map[int]*WorkerTask {
	if ws == nil {
		return map[int]*WorkerTask{}
	}

	ws.mu.RLock()
	defer ws.mu.RUnlock()

	tasks := make(map[int]*WorkerTask, len(ws.windows))
	for workerID := range ws.windows {
		tasks[workerID] = nil
	}
	for workerID, task := range ws.tasks {
		if task == nil {
			tasks[workerID] = nil
			continue
		}
		clone := *task
		tasks[workerID] = &clone
	}
	return tasks
}

func (ws *WorkerStats) Close() {
	if ws == nil || ws.aggregator == nil {
		return
	}
	ws.aggregator.Close()
}

func (ws *WorkerStats) addEvent(workerID int, eventTime time.Time) {
	if ws == nil {
		return
	}

	ws.mu.RLock()
	window, ok := ws.windows[workerID]
	ws.mu.RUnlock()

	if !ok {
		ws.mu.Lock()
		window, ok = ws.windows[workerID]
		if !ok {
			window = NewSlidingWindow(defaultSlidingWindowSize, defaultSlidingWindowRange)
			ws.windows[workerID] = window
		}
		if _, hasRef := ws.workerRefs[workerID]; !hasRef {
			ws.workerRefs[workerID] = inferWorkerRef(workerID)
		}
		ws.mu.Unlock()
	}

	window.Add(eventTime)
}

type ProgressAggregator struct {
	stats     *WorkerStats
	events    chan workerEvent
	done      chan struct{}
	closeOnce sync.Once
}

type workerEvent struct {
	workerID  int
	eventTime time.Time
}

func NewProgressAggregator(stats *WorkerStats) *ProgressAggregator {
	aggregator := &ProgressAggregator{
		stats:  stats,
		events: make(chan workerEvent, progressEventBufferSize),
		done:   make(chan struct{}),
	}
	go aggregator.run()
	return aggregator
}

func (pa *ProgressAggregator) Report(workerID int) {
	if pa == nil {
		return
	}
	event := workerEvent{workerID: workerID, eventTime: time.Now()}
	select {
	case pa.events <- event:
	default:
	}
}

func (pa *ProgressAggregator) Close() {
	if pa == nil {
		return
	}
	pa.closeOnce.Do(func() {
		close(pa.events)
		<-pa.done
	})
}

func (pa *ProgressAggregator) run() {
	defer close(pa.done)
	for event := range pa.events {
		pa.stats.addEvent(event.workerID, event.eventTime)
	}
}
