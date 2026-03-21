package checks

// Result is the outcome of a single check execution.
type Result struct {
	Monitor      string
	Status       string  // "up" or "down"
	LatencyMs    float64
	MetadataJSON string // JSON string with check-specific data
	Error        string // human-readable error for alerting (empty on success)
}
