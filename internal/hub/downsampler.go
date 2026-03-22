package hub

import (
	"log"
	"sync"
	"time"

	"github.com/andyhazz/whatsupp/internal/store"
)

// Retention periods are derived from tier thresholds. Each tier only needs
// to keep data long enough to cover the zoom levels it serves, plus headroom
// for aggregation lag. These are not user-configurable — they're implementation
// details of the tiered storage system.
const (
	retainCheckResultsRaw  = 12 * time.Hour         // raw ≤6h zoom + headroom
	retainAgentMetricsRaw  = 6 * time.Hour           // raw ≤1h zoom + headroom
	retainAgentMetrics5Min = 24 * time.Hour           // 5min ≤6h zoom + headroom
	retainAgentMetrics15Min = 7 * 24 * time.Hour      // 15min ≤48h zoom + headroom
	retainHourly           = 90 * 24 * time.Hour      // hourly ≤7d zoom + long lookback
	// daily = forever (no deletion)
)

// Downsampler performs periodic aggregation and cleanup.
type Downsampler struct {
	store  *store.Store
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewDownsampler creates a new downsampler.
func NewDownsampler(s *store.Store) *Downsampler {
	return &Downsampler{
		store:  s,
		stopCh: make(chan struct{}),
	}
}

// Start begins the downsampling goroutines.
func (d *Downsampler) Start() {
	d.wg.Add(4)
	go d.fiveMinLoop()
	go d.fifteenMinLoop()
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
			now := time.Now()
			end := now.Truncate(5 * time.Minute)
			start := end.Add(-5 * time.Minute)

			if err := d.store.AggregateAgentMetrics5Min(start, end); err != nil {
				log.Printf("downsampler: 5-min agent aggregation error: %v", err)
			}

			// Cleanup old raw agent metrics
			cutoff := now.Add(-retainAgentMetricsRaw)
			if n, err := d.store.DeleteOldAgentMetrics(cutoff); err != nil {
				log.Printf("downsampler: cleanup raw agent metrics error: %v", err)
			} else if n > 0 {
				log.Printf("downsampler: deleted %d old raw agent metrics", n)
			}
		case <-d.stopCh:
			return
		}
	}
}

func (d *Downsampler) fifteenMinLoop() {
	defer d.wg.Done()
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			end := now.Truncate(15 * time.Minute)
			start := end.Add(-15 * time.Minute)
			if err := d.store.AggregateAgentMetrics15Min(start, end); err != nil {
				log.Printf("downsampler: 15-min agent aggregation error: %v", err)
			}
		case <-d.stopCh:
			return
		}
	}
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

	now := time.Now()
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	timer := time.NewTimer(time.Until(nextMidnight))
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			d.runDailyAggregation()
			now := time.Now()
			nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			timer.Reset(time.Until(nextMidnight))
		case <-d.stopCh:
			return
		}
	}
}

func (d *Downsampler) runHourlyAggregation() {
	now := time.Now()
	hourEnd := now.Truncate(time.Hour)
	hourStart := hourEnd.Add(-time.Hour)

	// Aggregate check results
	if err := d.store.AggregateCheckResultsHourly(hourStart.Unix(), hourEnd.Unix()); err != nil {
		log.Printf("downsampler: hourly check aggregation error: %v", err)
	}

	// Cleanup old raw check results
	cutoff := now.Add(-retainCheckResultsRaw).Unix()
	if n, err := d.store.DeleteOldCheckResults(cutoff); err != nil {
		log.Printf("downsampler: cleanup raw check results error: %v", err)
	} else if n > 0 {
		log.Printf("downsampler: deleted %d old raw check results", n)
	}

	// Aggregate agent metrics hourly (from 5-min data)
	if err := d.store.AggregateAgentMetricsHourly(hourStart.Unix(), hourEnd.Unix()); err != nil {
		log.Printf("downsampler: hourly agent aggregation error: %v", err)
	}

	// Cleanup old 5-min and 15-min agent metrics
	agentCutoff5 := now.Add(-retainAgentMetrics5Min)
	if n, err := d.store.DeleteOldAgentMetrics5Min(agentCutoff5); err != nil {
		log.Printf("downsampler: cleanup 5min agent metrics error: %v", err)
	} else if n > 0 {
		log.Printf("downsampler: deleted %d old 5-min agent metrics", n)
	}
	agentCutoff15 := now.Add(-retainAgentMetrics15Min)
	if n, err := d.store.DeleteOldAgentMetrics15Min(agentCutoff15); err != nil {
		log.Printf("downsampler: cleanup 15min agent metrics error: %v", err)
	} else if n > 0 {
		log.Printf("downsampler: deleted %d old 15-min agent metrics", n)
	}
}

func (d *Downsampler) runDailyAggregation() {
	now := time.Now()
	dayEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayStart := dayEnd.AddDate(0, 0, -1)

	if err := d.store.AggregateCheckResultsDaily(dayStart.Unix(), dayEnd.Unix()); err != nil {
		log.Printf("downsampler: daily check aggregation error: %v", err)
	}

	if err := d.store.AggregateAgentMetricsDaily(dayStart.Unix(), dayEnd.Unix()); err != nil {
		log.Printf("downsampler: daily agent aggregation error: %v", err)
	}

	// Cleanup old hourly data
	hourlyCutoff := now.Add(-retainHourly).Unix()
	if n, err := d.store.DeleteOldHourlyCheckResults(hourlyCutoff); err != nil {
		log.Printf("downsampler: cleanup hourly error: %v", err)
	} else if n > 0 {
		log.Printf("downsampler: deleted %d old hourly check results", n)
	}
}
