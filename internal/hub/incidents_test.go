package hub

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/andyhazz/whatsupp/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open() error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestIncidentManager_CreateOnDown(t *testing.T) {
	s := testStore(t)
	im := NewIncidentManager(s)

	now := time.Now().Unix()
	inc, err := im.HandleTransition("Plex", TransitionToDown, now, "connection refused")
	if err != nil {
		t.Fatalf("HandleTransition() error: %v", err)
	}
	if inc == nil {
		t.Fatal("HandleTransition() returned nil incident on DOWN transition")
	}
	if inc.Cause != "connection refused" {
		t.Errorf("Cause = %q, want %q", inc.Cause, "connection refused")
	}
}

func TestIncidentManager_ResolveOnUp(t *testing.T) {
	s := testStore(t)
	im := NewIncidentManager(s)

	now := time.Now().Unix()
	_, err := im.HandleTransition("Plex", TransitionToDown, now, "timeout")
	if err != nil {
		t.Fatalf("HandleTransition(DOWN) error: %v", err)
	}

	inc, err := im.HandleTransition("Plex", TransitionToUp, now+300, "")
	if err != nil {
		t.Fatalf("HandleTransition(UP) error: %v", err)
	}
	if inc == nil {
		t.Fatal("HandleTransition(UP) returned nil incident")
	}
	if inc.ResolvedAt == nil {
		t.Fatal("ResolvedAt should be set after resolve")
	}
	if *inc.ResolvedAt != now+300 {
		t.Errorf("ResolvedAt = %d, want %d", *inc.ResolvedAt, now+300)
	}
}

func TestIncidentManager_NoOpOnNone(t *testing.T) {
	s := testStore(t)
	im := NewIncidentManager(s)

	now := time.Now().Unix()
	inc, err := im.HandleTransition("Plex", TransitionNone, now, "")
	if err != nil {
		t.Fatalf("HandleTransition(None) error: %v", err)
	}
	if inc != nil {
		t.Error("HandleTransition(None) should return nil")
	}
}

func TestIncidentManager_ResolveNoOpen(t *testing.T) {
	s := testStore(t)
	im := NewIncidentManager(s)

	now := time.Now().Unix()
	inc, err := im.HandleTransition("Plex", TransitionToUp, now, "")
	if err != nil {
		t.Fatalf("HandleTransition(UP) error: %v", err)
	}
	if inc != nil {
		t.Error("HandleTransition(UP) with no open incident should return nil")
	}
}
