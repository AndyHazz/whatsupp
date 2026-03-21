package hub

import (
	"log"
	"sync"
	"time"

	"github.com/andyhazz/whatsupp/internal/store"
)

// RetentionConfig defines how long each tier is kept.
type RetentionConfig struct {
	CheckResultsRaw  time.Duration
	AgentMetricsRaw  time.Duration
	AgentMetrics5Min time.Duration
	Hourly           time.Duration
	// Daily is forever (no deletion)
}

// DefaultRetentionConfig returns spec defaults.
func DefaultRetentionConfig() RetentionConfig {
	return RetentionConfig{
		CheckResultsRaw:  30 * 24 * time.Hour,  // 30 days
		AgentMetricsRaw:  48 * time.Hour,        // 48 hours
		AgentMetrics5Min: 90 * 24 * time.Hour,   // 90 days
		Hourly:           180 * 24 * time.Hour,   // 6 months
	}
}

// Downsampler performs periodic aggregation and cleanup.
type Downsampler struct {
	store     *store.Store
	retention RetentionConfig
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewDownsampler creates a new downsampler.
func NewDownsampler(s *store.Store, retention RetentionConfig) *Downsampler {
	return &Downsampler{
		store:     s,
		retention: retention,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the downsampling goroutines.
func (d *Downsampler) Start() {
	d.wg.Add(3)
	go d.fiveMinLoop()
	go d.hourlyLoop()
	go d.dailyLoop()
}

// Stop signals downsampling goroutines to stop and waits.
func (d *Downsampler) Stop() {
	close(d.stopCh)
	d.wg.Wait()
}

func (d *Downsampler) fiveMinLoop() {
	defer d.wg.Done()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.runFiveMinAggregation()
		case <-d.stopCh:
			return
		}
	}
}

func (d *Downsampler) runFiveMinAggregation() {
	now := time.Now()
	// Aggregate the previous 5 minutes
	end := now.Truncate(5 * time.Minute)
	start := end.Add(-5 * time.Minute)

	if err := d.AggregateAgentMetrics5Min(start, end); err != nil {
		log.Printf("downsampler: 5-min agent aggregation error: %v", err)
	}

	// Cleanup old raw agent metrics
	if d.retention.AgentMetricsRaw > 0 {
		cutoff := now.Add(-d.retention.AgentMetricsRaw)
		if n, err := d.PurgeRawAgentMetrics(cutoff); err != nil {
			log.Printf("downsampler: cleanup raw agent metrics error: %v", err)
		} else if n > 0 {
			log.Printf("downsampler: deleted %d old raw agent metrics", n)
		}
	}
}

// AggregateAgentMetrics5Min aggregates raw agent metrics into 5-minute buckets.
func (d *Downsampler) AggregateAgentMetrics5Min(start, end time.Time) error {
	return d.store.AggregateAgentMetrics5Min(start, end)
}

// PurgeRawAgentMetrics deletes raw agent metrics older than the given time.
func (d *Downsampler) PurgeRawAgentMetrics(olderThan time.Time) (int64, error) {
	return d.store.DeleteOldAgentMetrics(olderThan)
}

func (d *Downsampler) hourlyLoop() {
	defer d.wg.Done()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.runHourlyAggregation()
		case <-d.stopCh:
			return
		}
	}
}

func (d *Downsampler) dailyLoop() {
	defer d.wg.Done()

	// Calculate time until next midnight
	now := time.Now()
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	timer := time.NewTimer(time.Until(nextMidnight))
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			d.runDailyAggregation()
			// Reset timer for next midnight
			now := time.Now()
			nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			timer.Reset(time.Until(nextMidnight))
		case <-d.stopCh:
			return
		}
	}
}

func (d *Downsampler) runHourlyAggregation() {
	// Aggregate the previous hour
	now := time.Now()
	hourEnd := now.Truncate(time.Hour)
	hourStart := hourEnd.Add(-time.Hour)

	if err := d.AggregateHour(hourStart.Unix()); err != nil {
		log.Printf("downsampler: hourly aggregation error: %v", err)
	}

	// Cleanup old raw check results
	if n, err := d.CleanupRawCheckResults(); err != nil {
		log.Printf("downsampler: cleanup raw check results error: %v", err)
	} else if n > 0 {
		log.Printf("downsampler: deleted %d old raw check results", n)
	}

	// Aggregate agent metrics hourly (from 5-min data)
	if err := d.store.AggregateAgentMetricsHourly(hourStart.Unix(), hourEnd.Unix()); err != nil {
		log.Printf("downsampler: hourly agent aggregation error: %v", err)
	}

	// Cleanup old 5-min agent metrics
	if d.retention.AgentMetrics5Min > 0 {
		cutoff := now.Add(-d.retention.AgentMetrics5Min)
		if n, err := d.store.DeleteOldAgentMetrics5Min(cutoff); err != nil {
			log.Printf("downsampler: cleanup 5min agent metrics error: %v", err)
		} else if n > 0 {
			log.Printf("downsampler: deleted %d old 5-min agent metrics", n)
		}
	}
}

func (d *Downsampler) runDailyAggregation() {
	// Aggregate the previous day from hourly
	now := time.Now()
	dayEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayStart := dayEnd.AddDate(0, 0, -1)

	if err := d.store.AggregateCheckResultsDaily(dayStart.Unix(), dayEnd.Unix()); err != nil {
		log.Printf("downsampler: daily check aggregation error: %v", err)
	}

	// Aggregate agent metrics daily (from hourly)
	if err := d.store.AggregateAgentMetricsDaily(dayStart.Unix(), dayEnd.Unix()); err != nil {
		log.Printf("downsampler: daily agent aggregation error: %v", err)
	}

	// Cleanup old hourly data
	cutoff := now.Add(-d.retention.Hourly).Unix()
	if n, err := d.store.DeleteOldHourlyCheckResults(cutoff); err != nil {
		log.Printf("downsampler: cleanup hourly error: %v", err)
	} else if n > 0 {
		log.Printf("downsampler: deleted %d old hourly check results", n)
	}
}

// AggregateHour aggregates raw check results for the hour starting at hourStart.
func (d *Downsampler) AggregateHour(hourStartUnix int64) error {
	return d.store.AggregateCheckResultsHourly(hourStartUnix, hourStartUnix+3600)
}

// CleanupRawCheckResults deletes raw check results older than retention period.
func (d *Downsampler) CleanupRawCheckResults() (int64, error) {
	cutoff := time.Now().Add(-d.retention.CheckResultsRaw).Unix()
	return d.store.DeleteOldCheckResults(cutoff)
}
