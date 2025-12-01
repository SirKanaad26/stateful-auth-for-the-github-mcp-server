package performance

import (
	"fmt"
	"sync"
	"time"
)

// Metrics tracks performance metrics for MCP API calls
type Metrics struct {
	mu                sync.RWMutex
	toolCallDurations []time.Duration
	startTime         time.Time
	totalCalls        int64
	statefulAuthCalls int64
	noAuthCalls       int64
}

// NewMetrics creates a new performance metrics tracker
func NewMetrics() *Metrics {
	return &Metrics{
		toolCallDurations: make([]time.Duration, 0, 1000),
		startTime:         time.Now(),
	}
}

// RecordToolCall records the duration of a tool call
func (m *Metrics) RecordToolCall(duration time.Duration, isStatefulAuth bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.toolCallDurations = append(m.toolCallDurations, duration)
	m.totalCalls++

	if isStatefulAuth {
		m.statefulAuthCalls++
	} else {
		m.noAuthCalls++
	}
}

// GetStats returns statistics about recorded metrics
func (m *Metrics) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.toolCallDurations) == 0 {
		return Stats{}
	}

	// Calculate statistics
	var total time.Duration
	min := m.toolCallDurations[0]
	max := m.toolCallDurations[0]

	for _, d := range m.toolCallDurations {
		total += d
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}

	avg := total / time.Duration(len(m.toolCallDurations))

	// Calculate median
	sortedDurations := make([]time.Duration, len(m.toolCallDurations))
	copy(sortedDurations, m.toolCallDurations)
	quickSort(sortedDurations, 0, len(sortedDurations)-1)

	var median time.Duration
	n := len(sortedDurations)
	if n%2 == 0 {
		median = (sortedDurations[n/2-1] + sortedDurations[n/2]) / 2
	} else {
		median = sortedDurations[n/2]
	}

	// Calculate percentiles
	p50 := sortedDurations[n/2]
	p95 := sortedDurations[int(float64(n)*0.95)]
	p99 := sortedDurations[int(float64(n)*0.99)]

	return Stats{
		TotalCalls:        m.totalCalls,
		StatefulAuthCalls: m.statefulAuthCalls,
		NoAuthCalls:       m.noAuthCalls,
		Min:               min,
		Max:               max,
		Average:           avg,
		Median:            median,
		P50:               p50,
		P95:               p95,
		P99:               p99,
		TotalDuration:     time.Since(m.startTime),
		SampleSize:        len(m.toolCallDurations),
	}
}

// Reset clears all recorded metrics
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.toolCallDurations = make([]time.Duration, 0, 1000)
	m.startTime = time.Now()
	m.totalCalls = 0
	m.statefulAuthCalls = 0
	m.noAuthCalls = 0
}

// GetRawDurations returns a copy of all recorded durations
func (m *Metrics) GetRawDurations() []time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	durations := make([]time.Duration, len(m.toolCallDurations))
	copy(durations, m.toolCallDurations)
	return durations
}

// Stats contains performance statistics
type Stats struct {
	TotalCalls        int64
	StatefulAuthCalls int64
	NoAuthCalls       int64
	Min               time.Duration
	Max               time.Duration
	Average           time.Duration
	Median            time.Duration
	P50               time.Duration
	P95               time.Duration
	P99               time.Duration
	TotalDuration     time.Duration
	SampleSize        int
}

// String returns a formatted string representation of the stats
func (s Stats) String() string {
	return fmt.Sprintf(`Performance Statistics:
  Total Calls:           %d
  - With Stateful Auth:  %d
  - Without Auth:        %d
  Sample Size:           %d
  
  Latency:
    Min:                 %v
    Max:                 %v
    Average:             %v
    Median:              %v
    P50:                 %v
    P95:                 %v
    P99:                 %v
  
  Total Test Duration:   %v
  Average Throughput:    %.2f calls/sec`,
		s.TotalCalls,
		s.StatefulAuthCalls,
		s.NoAuthCalls,
		s.SampleSize,
		s.Min,
		s.Max,
		s.Average,
		s.Median,
		s.P50,
		s.P95,
		s.P99,
		s.TotalDuration,
		float64(s.TotalCalls)/s.TotalDuration.Seconds(),
	)
}

// quickSort is a helper function for sorting durations
func quickSort(arr []time.Duration, low, high int) {
	if low < high {
		pi := partition(arr, low, high)
		quickSort(arr, low, pi-1)
		quickSort(arr, pi+1, high)
	}
}

func partition(arr []time.Duration, low, high int) int {
	pivot := arr[high]
	i := low - 1

	for j := low; j < high; j++ {
		if arr[j] < pivot {
			i++
			arr[i], arr[j] = arr[j], arr[i]
		}
	}
	arr[i+1], arr[high] = arr[high], arr[i+1]
	return i + 1
}
