package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/andyhazz/whatsupp/internal/version"
)

// maxCollectorBackoff caps how long a persistently failing collector is skipped
// for. Long enough that a permanently broken collector costs almost nothing,
// short enough that one which starts working again is picked up the same day.
const maxCollectorBackoff = 30 * time.Minute

// Agent collects system metrics and pushes them to the hub.
type Agent struct {
	config     *AgentConfig
	collectors []Collector
	push       *PushClient
	buffer     *MetricBuffer
	hostname   string
	backoff    *Backoff
}

// New creates a new Agent from config.
func New(cfg *AgentConfig) (*Agent, error) {
	// Setup host filesystem paths for containerized collection
	SetupHostFS(cfg.HostFS)

	// Create collectors, omitting any the config disables
	var collectors []Collector
	for _, c := range []Collector{
		NewCPUCollector(),
		NewMemCollector(),
		NewDiskCollector(),
		NewNetCollector(),
		NewTempCollector(),
		NewDockerCollector(cfg.DockerHost),
		NewBatteryCollector(),
	} {
		if cfg.CollectorEnabled(c.Name()) {
			collectors = append(collectors, c)
		} else {
			log.Printf("agent: collector %s disabled by config", c.Name())
		}
	}

	return &Agent{
		config:     cfg,
		collectors: collectors,
		push:       NewPushClient(cfg.HubURL, cfg.AgentKey),
		buffer:     NewMetricBuffer(5*time.Minute, 10),
		hostname:   cfg.Hostname,
		backoff:    NewBackoff(cfg.Interval, maxCollectorBackoff),
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

	now := time.Now()
	for _, c := range a.collectors {
		// Skip collectors that are backing off after repeated failures
		if !a.backoff.Allow(c.Name(), now) {
			continue
		}
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
			// Only the first failure of a run is logged; the rest would be
			// identical, and the collector is now being retried less often.
			if a.backoff.Failure(r.name, now) {
				log.Printf("agent: collector %s failing, backing off: %v", r.name, r.err)
			}
			continue
		}
		if a.backoff.Success(r.name) {
			log.Printf("agent: collector %s recovered", r.name)
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
