package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/andyhazz/whatsupp/internal/version"
)

// Agent collects system metrics and pushes them to the hub.
type Agent struct {
	config     *AgentConfig
	collectors []Collector
	push       *PushClient
	buffer     *MetricBuffer
	hostname   string
}

// New creates a new Agent from config.
func New(cfg *AgentConfig) (*Agent, error) {
	// Setup host filesystem paths for containerized collection
	SetupHostFS(cfg.HostFS)

	// Create collectors
	collectors := []Collector{
		NewCPUCollector(),
		NewMemCollector(),
		NewDiskCollector(),
		NewNetCollector(),
		NewTempCollector(),
		NewDockerCollector(cfg.DockerHost),
		NewBatteryCollector(),
	}

	return &Agent{
		config:     cfg,
		collectors: collectors,
		push:       NewPushClient(cfg.HubURL, cfg.AgentKey),
		buffer:     NewMetricBuffer(5*time.Minute, 10),
		hostname:   cfg.Hostname,
	}, nil
}

// Run starts the agent collection loop. Blocks until ctx is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	ticker := time.NewTicker(a.config.Interval)
	defer ticker.Stop()

	// Run immediately on start
	a.collectAndPush(ctx)

	for {
		select {
		case <-ticker.C:
			a.collectAndPush(ctx)
		case <-ctx.Done():
			// Final flush attempt
			a.flushBuffer(context.Background())
			return nil
		}
	}
}

func (a *Agent) collectAndPush(ctx context.Context) {
	metrics := a.collect(ctx)
	batch := MetricBatch{
		Host:      a.hostname,
		Timestamp: time.Now(),
		Metrics:   metrics,
		Version:   version.Version,
	}

	// Try to flush buffered batches first
	a.flushBuffer(ctx)

	// Send current batch
	if err := a.push.Send(ctx, batch); err != nil {
		log.Printf("agent: push failed: %v (buffering)", err)
		a.buffer.Add(batch)
	} else {
		log.Printf("agent: pushed %d metrics", len(metrics))
	}
}

func (a *Agent) collect(ctx context.Context) []Metric {
	type result struct {
		metrics []Metric
		err     error
		name    string
	}

	results := make(chan result, len(a.collectors))
	var wg sync.WaitGroup

	for _, c := range a.collectors {
		wg.Add(1)
		go func(c Collector) {
			defer wg.Done()
			m, err := c.Collect(ctx)
			results <- result{metrics: m, err: err, name: c.Name()}
		}(c)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var all []Metric
	for r := range results {
		if r.err != nil {
			log.Printf("agent: collector %s error: %v", r.name, r.err)
			continue
		}
		all = append(all, r.metrics...)
	}

	return all
}

func (a *Agent) flushBuffer(ctx context.Context) {
	batches := a.buffer.Drain()
	for _, batch := range batches {
		if err := a.push.Send(ctx, batch); err != nil {
			log.Printf("agent: flush failed: %v (re-buffering %d batches)", err, len(batches))
			// Re-buffer remaining batches
			a.buffer.Add(batch)
			return
		}
	}
}
