package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testDB(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpen(t *testing.T) {
	s := testDB(t)
	// Verify WAL mode
	var journalMode string
	err := s.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("PRAGMA journal_mode error: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}
}

func TestInsertCheckResult(t *testing.T) {
	s := testDB(t)
	now := time.Now().Unix()
	err := s.InsertCheckResult("Plex", now, "up", 45.2, `{"status_code":200}`)
	if err != nil {
		t.Fatalf("InsertCheckResult() error: %v", err)
	}

	results, err := s.GetCheckResults("Plex", now-60, now+60)
	if err != nil {
		t.Fatalf("GetCheckResults() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Status != "up" {
		t.Errorf("Status = %q, want %q", results[0].Status, "up")
	}
	if results[0].LatencyMs != 45.2 {
		t.Errorf("LatencyMs = %f, want 45.2", results[0].LatencyMs)
	}
}

func TestInsertAndResolveIncident(t *testing.T) {
	s := testDB(t)
	now := time.Now().Unix()
	id, err := s.CreateIncident("Plex", now, "connection refused")
	if err != nil {
		t.Fatalf("CreateIncident() error: %v", err)
	}
	if id == 0 {
		t.Fatal("CreateIncident() returned id 0")
	}

	err = s.ResolveIncident(id, now+300)
	if err != nil {
		t.Fatalf("ResolveIncident() error: %v", err)
	}

	inc, err := s.GetOpenIncident("Plex")
	if err != nil {
		t.Fatalf("GetOpenIncident() error: %v", err)
	}
	if inc != nil {
		t.Error("GetOpenIncident() should return nil after resolve")
	}
}

func TestGetOpenIncident(t *testing.T) {
	s := testDB(t)
	now := time.Now().Unix()
	id, err := s.CreateIncident("Plex", now, "timeout")
	if err != nil {
		t.Fatalf("CreateIncident() error: %v", err)
	}

	inc, err := s.GetOpenIncident("Plex")
	if err != nil {
		t.Fatalf("GetOpenIncident() error: %v", err)
	}
	if inc == nil {
		t.Fatal("GetOpenIncident() returned nil, want incident")
	}
	if inc.ID != id {
		t.Errorf("ID = %d, want %d", inc.ID, id)
	}
	if inc.Cause != "timeout" {
		t.Errorf("Cause = %q, want %q", inc.Cause, "timeout")
	}
}

func TestInsertSecurityScan(t *testing.T) {
	s := testDB(t)
	now := time.Now().Unix()
	err := s.InsertSecurityScan("203.0.113.1", now, `[22,80,443]`)
	if err != nil {
		t.Fatalf("InsertSecurityScan() error: %v", err)
	}
}

func TestSecurityBaseline(t *testing.T) {
	s := testDB(t)
	now := time.Now().Unix()
	err := s.UpsertSecurityBaseline("203.0.113.1", `[22,80,443]`, now)
	if err != nil {
		t.Fatalf("UpsertSecurityBaseline() error: %v", err)
	}

	bl, err := s.GetSecurityBaseline("203.0.113.1")
	if err != nil {
		t.Fatalf("GetSecurityBaseline() error: %v", err)
	}
	if bl == nil {
		t.Fatal("GetSecurityBaseline() returned nil")
	}
	if bl.ExpectedPortsJSON != `[22,80,443]` {
		t.Errorf("ExpectedPortsJSON = %q, want %q", bl.ExpectedPortsJSON, `[22,80,443]`)
	}
}

func TestDeleteOldCheckResults(t *testing.T) {
	s := testDB(t)
	now := time.Now().Unix()
	old := now - 86400*31 // 31 days ago

	s.InsertCheckResult("Plex", old, "up", 40.0, "")
	s.InsertCheckResult("Plex", now, "up", 50.0, "")

	cutoff := now - 86400*30 // 30 days
	n, err := s.DeleteOldCheckResults(cutoff)
	if err != nil {
		t.Fatalf("DeleteOldCheckResults() error: %v", err)
	}
	if n != 1 {
		t.Errorf("deleted = %d, want 1", n)
	}

	results, err := s.GetCheckResults("Plex", 0, now+60)
	if err != nil {
		t.Fatalf("GetCheckResults() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("remaining results = %d, want 1", len(results))
	}
}

func TestAggregateHourly(t *testing.T) {
	s := testDB(t)
	// Insert 3 check results in the same hour
	base := int64(1711018800) // some fixed hour boundary
	s.InsertCheckResult("Plex", base+10, "up", 40.0, "")
	s.InsertCheckResult("Plex", base+20, "up", 60.0, "")
	s.InsertCheckResult("Plex", base+30, "down", 0.0, "")

	err := s.AggregateCheckResultsHourly(base, base+3600)
	if err != nil {
		t.Fatalf("AggregateCheckResultsHourly() error: %v", err)
	}

	rows, err := s.GetCheckResultsHourly("Plex", base, base+3600)
	if err != nil {
		t.Fatalf("GetCheckResultsHourly() error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(hourly) = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.SuccessCount != 2 {
		t.Errorf("SuccessCount = %d, want 2", r.SuccessCount)
	}
	if r.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", r.FailCount)
	}
}

func TestDBFileCreated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "test.db")
	// Subdir doesn't exist; Open should create it
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	s.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("DB file was not created")
	}
}
