package agent

import (
	"sync"
	"testing"
	"time"
)

func TestBuffer_Add(t *testing.T) {
	b := NewMetricBuffer(5*time.Minute, 10)
	b.Add(MetricBatch{Host: "h", Timestamp: time.Now()})
	if b.Len() != 1 {
		t.Errorf("Len() = %d, want 1", b.Len())
	}
}

func TestBuffer_Drain(t *testing.T) {
	b := NewMetricBuffer(5*time.Minute, 10)
	b.Add(MetricBatch{Host: "h1", Timestamp: time.Now()})
	b.Add(MetricBatch{Host: "h2", Timestamp: time.Now()})

	batches := b.Drain()
	if len(batches) != 2 {
		t.Fatalf("Drain() returned %d batches, want 2", len(batches))
	}
	if batches[0].Host != "h1" {
		t.Error("expected FIFO order")
	}
	if batches[1].Host != "h2" {
		t.Error("expected FIFO order")
	}

	// Buffer should be empty after drain
	if b.Len() != 0 {
		t.Errorf("Len() after drain = %d, want 0", b.Len())
	}
}

func TestBuffer_MaxAge(t *testing.T) {
	b := NewMetricBuffer(100*time.Millisecond, 10)
	b.Add(MetricBatch{Host: "old", Timestamp: time.Now()})
	time.Sleep(200 * time.Millisecond)
	b.Add(MetricBatch{Host: "new", Timestamp: time.Now()})

	// Old batch should be evicted
	if b.Len() != 1 {
		t.Errorf("Len() = %d, want 1 (old batch evicted)", b.Len())
	}
}

func TestBuffer_MaxSize(t *testing.T) {
	b := NewMetricBuffer(5*time.Minute, 3)
	for i := 0; i < 5; i++ {
		b.Add(MetricBatch{Host: "h", Timestamp: time.Now()})
	}

	if b.Len() != 3 {
		t.Errorf("Len() = %d, want 3 (maxSize)", b.Len())
	}
}

func TestBuffer_Concurrent(t *testing.T) {
	b := NewMetricBuffer(5*time.Minute, 100)
	var wg sync.WaitGroup

	// Concurrent adds
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				b.Add(MetricBatch{Host: "h", Timestamp: time.Now()})
			}
		}()
	}

	// Concurrent drains
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Drain()
		}()
	}

	wg.Wait()
	// Should not panic — exact count doesn't matter
}
