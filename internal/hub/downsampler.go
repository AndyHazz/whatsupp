package hub

import (
	"log"
	"sync"
	"time"

	"github.com/andyhazz/whatsupp/internal/store"
)

// RetentionConfig defines how long each tier is kept.
type RetentionConfig struct {
	CheckResultsRaw time.Duration
	Hourly           time.Duration
	// Daily is forever (no deletion)
}

// DefaultRetentionConfig returns spec defaults.
func DefaultRetentionConfig() RetentionConfig {
	return RetentionConfig{
		CheckResultsRaw: 30 * 24 * time.Hour,  // 30 days
		Hourly:          180 * 24 * time.Hour,  // 6 months
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
	d.wg.Add(2)
	go d.hourlyLoop()
	go d.dailyLoop()
}

// Stop signals downsampling goroutines to stop and waits.
func (d *Downsampler) Stop() {
	close(d.stopCh)
	d.wg.Wait()
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
}

func (d *Downsampler) runDailyAggregation() {
	// Aggregate the previous day from hourly
	now := time.Now()
	dayEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayStart := dayEnd.AddDate(0, 0, -1)

	if err := d.store.AggregateCheckResultsDaily(dayStart.Unix(), dayEnd.Unix()); err != nil {
		log.Printf("downsampler: daily aggregation error: %v", err)
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
