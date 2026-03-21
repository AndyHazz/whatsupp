package checks

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

// PrometheusMetric represents a single metric from Prometheus text format.
type PrometheusMetric struct {
	Name   string
	Labels map[string]string
	Value  float64
}

// ParsePrometheusText parses Prometheus exposition format text into metrics.
func ParsePrometheusText(reader io.Reader) ([]PrometheusMetric, error) {
	parser := expfmt.NewTextParser(model.UTF8Validation)
	families, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		// expfmt may return partial results alongside errors
		if len(families) == 0 {
			return nil, fmt.Errorf("parse prometheus text: %w", err)
		}
	}

	var metrics []PrometheusMetric
	for name, family := range families {
		for _, m := range family.GetMetric() {
			labels := make(map[string]string)
			for _, lp := range m.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}

			var value float64
			switch family.GetType() {
			case dto.MetricType_COUNTER:
				value = m.GetCounter().GetValue()
			case dto.MetricType_GAUGE:
				value = m.GetGauge().GetValue()
			case dto.MetricType_UNTYPED:
				value = m.GetUntyped().GetValue()
			default:
				continue // skip histograms, summaries
			}

			metrics = append(metrics, PrometheusMetric{
				Name:   name,
				Labels: labels,
				Value:  value,
			})
		}
	}

	return metrics, nil
}

// ScrapeMetric is a metric with a name and value, matching the agent naming convention.
type ScrapeMetric struct {
	Name  string
	Value float64
}

// ScrapeCheck scrapes a Prometheus endpoint and maps metrics to whatsupp naming.
type ScrapeCheck struct {
	name   string
	url    string
	mapper *NodeExporterMapper
	client *http.Client
}

// NewScrapeCheck creates a new scrape check.
func NewScrapeCheck(name, url string) *ScrapeCheck {
	return &ScrapeCheck{
		name:   name,
		url:    url,
		mapper: NewNodeExporterMapper(),
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Execute performs a scrape and returns mapped metrics.
func (s *ScrapeCheck) Execute(ctx context.Context) ([]ScrapeMetric, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scrape %s: %w", s.url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scrape %s: status %d", s.url, resp.StatusCode)
	}

	promMetrics, err := ParsePrometheusText(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse metrics from %s: %w", s.url, err)
	}

	mapped := s.mapper.Map(promMetrics)

	// Convert to ScrapeMetric
	result := make([]ScrapeMetric, len(mapped))
	for i, m := range mapped {
		result[i] = ScrapeMetric{Name: m.Name, Value: m.Value}
	}

	return result, nil
}

// MappedMetric represents a metric after mapping from Prometheus to whatsupp naming.
type MappedMetric struct {
	Name  string
	Value float64
}

// isFilteredMount returns true if a mountpoint should be excluded.
func isFilteredMount(mount string) bool {
	filtered := []string{"/sys", "/proc", "/dev", "/run", "/snap"}
	for _, prefix := range filtered {
		if mount == prefix || strings.HasPrefix(mount, prefix+"/") {
			return true
		}
	}
	return false
}

// isFilteredIface returns true if a network interface should be excluded.
func isFilteredIface(iface string) bool {
	if iface == "lo" {
		return true
	}
	if strings.HasPrefix(iface, "veth") || strings.HasPrefix(iface, "docker") || strings.HasPrefix(iface, "br-") {
		return true
	}
	return false
}
