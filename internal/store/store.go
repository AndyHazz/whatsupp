package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// CheckResult represents a single check result row.
type CheckResult struct {
	Monitor      string
	Timestamp    int64
	Status       string
	LatencyMs    float64
	MetadataJSON string
}

// CheckResultHourly represents an hourly aggregation.
type CheckResultHourly struct {
	Monitor      string
	Hour         int64
	AvgLatency   float64
	MinLatency   float64
	MaxLatency   float64
	SuccessCount int
	FailCount    int
	UptimePct    float64
}

// Incident represents an incident row.
type Incident struct {
	ID         int64
	Monitor    string
	StartedAt  int64
	ResolvedAt *int64
	Cause      string
}

// SecurityBaseline represents a security baseline row.
type SecurityBaseline struct {
	Target            string
	ExpectedPortsJSON string
	UpdatedAt         int64
}

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

// Open creates or opens the SQLite database and runs migrations.
func Open(path string) (*Store, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Run schema migration
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for advanced use.
func (s *Store) DB() *sql.DB {
	return s.db
}
