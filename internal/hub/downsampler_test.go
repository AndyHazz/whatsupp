package hub

import (
	"testing"
	"time"
)

func TestDownsampler_HourlyAggregation(t *testing.T) {
	s := testStore(t)

	// Insert check results across two hours
	hour1 := int64(1711000000) // some epoch, truncated to hour
	hour1 = hour1 - (hour1 % 3600)
	hour2 := hour1 + 3600

	// Hour 1: 2 up, 1 down
	s.InsertCheckResult("Plex", hour1+10, "up", 40.0, "")
	s.InsertCheckResult("Plex", hour1+20, "up", 60.0, "")
	s.InsertCheckResult("Plex", hour1+30, "down", 0.0, "")

	// Hour 2: 3 up
	s.InsertCheckResult("Plex", hour2+10, "up", 30.0, "")
	s.InsertCheckResult("Plex", hour2+20, "up", 50.0, "")
	s.InsertCheckResult("Plex", hour2+30, "up", 70.0, "")

	d := NewDownsampler(s, DefaultRetentionConfig())

	// Aggregate hour 1
	err := d.AggregateHour(hour1)
	if err != nil {
		t.Fatalf("AggregateHour() error: %v", err)
	}

	rows, err := s.GetCheckResultsHourly("Plex", hour1, hour1+3600)
	if err != nil {
		t.Fatalf("GetCheckResultsHourly() error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("hourly rows = %d, want 1", len(rows))
	}
	if rows[0].SuccessCount != 2 {
		t.Errorf("SuccessCount = %d, want 2", rows[0].SuccessCount)
	}
	if rows[0].FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", rows[0].FailCount)
	}
}

func TestDownsampler_Cleanup(t *testing.T) {
	s := testStore(t)

	now := time.Now().Unix()
	old := now - 86400*31 // 31 days ago

	s.InsertCheckResult("Plex", old, "up", 40.0, "")
	s.InsertCheckResult("Plex", now, "up", 50.0, "")

	d := NewDownsampler(s, DefaultRetentionConfig())
	n, err := d.CleanupRawCheckResults()
	if err != nil {
		t.Fatalf("CleanupRawCheckResults() error: %v", err)
	}
	if n != 1 {
		t.Errorf("cleaned up = %d, want 1", n)
	}
}

func TestDefaultRetentionConfig(t *testing.T) {
	rc := DefaultRetentionConfig()
	if rc.CheckResultsRaw != 30*24*time.Hour {
		t.Errorf("CheckResultsRaw = %v, want 720h", rc.CheckResultsRaw)
	}
	if rc.Hourly != 180*24*time.Hour {
		t.Errorf("Hourly = %v, want 4320h", rc.Hourly)
	}
}
