package agent

import (
	"sync"
	"time"
)

// MetricBuffer provides a thread-safe buffer for metric batches.
type MetricBuffer struct {
	mu      sync.Mutex
	batches []MetricBatch
	maxAge  time.Duration
	maxSize int
}

// NewMetricBuffer creates a new metric buffer.
func NewMetricBuffer(maxAge time.Duration, maxSize int) *MetricBuffer {
	return &MetricBuffer{
		maxAge:  maxAge,
		maxSize: maxSize,
	}
}

// Add adds a batch to the buffer. Evicts old and excess entries.
func (b *MetricBuffer) Add(batch MetricBatch) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Evict batches older than maxAge
	cutoff := time.Now().Add(-b.maxAge)
	valid := b.batches[:0]
	for _, existing := range b.batches {
		if existing.Timestamp.After(cutoff) {
			valid = append(valid, existing)
		}
	}
	b.batches = valid

	// Add new batch
	b.batches = append(b.batches, batch)

	// If over maxSize, drop oldest
	if len(b.batches) > b.maxSize {
		b.batches = b.batches[len(b.batches)-b.maxSize:]
	}
}

// Drain returns all buffered batches in FIFO order and empties the buffer.
func (b *MetricBuffer) Drain() []MetricBatch {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.batches) == 0 {
		return nil
	}

	result := make([]MetricBatch, len(b.batches))
	copy(result, b.batches)
	b.batches = b.batches[:0]
	return result
}

// Len returns the number of buffered batches.
func (b *MetricBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.batches)
}
