package agent

import "context"

// Collector is the interface for all metric collectors.
type Collector interface {
	Name() string
	Collect(ctx context.Context) ([]Metric, error)
}
