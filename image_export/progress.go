package image_export

import (
	"sync"
	"time"
)

const (
	defaultSlidingWindowSize  = 1024
	defaultSlidingWindowRange = 30 * time.Second
	progressEventBufferSize   = 100
)

type ProgressUpdate struct {
	WorkerCounts map[int]int
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
	aggregator *ProgressAggregator
}

func NewWorkerStats(workerCount int) *WorkerStats {
	if workerCount < 1 {
		workerCount = 1
	}

	stats := &WorkerStats{
		windows: make(map[int]*SlidingWindow, workerCount),
	}
	for workerID := 0; workerID < workerCount; workerID++ {
		stats.windows[workerID] = NewSlidingWindow(defaultSlidingWindowSize, defaultSlidingWindowRange)
	}
	stats.aggregator = NewProgressAggregator(stats)
	return stats
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
