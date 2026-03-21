package agent

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
)

var virtualFS = map[string]bool{
	"tmpfs": true, "devtmpfs": true, "sysfs": true, "proc": true,
	"devpts": true, "securityfs": true, "cgroup": true, "cgroup2": true,
	"pstore": true, "efivarfs": true, "bpf": true, "tracefs": true,
	"debugfs": true, "hugetlbfs": true, "mqueue": true, "fusectl": true,
	"overlay": true, "nsfs": true, "fuse.lxcfs": true, "squashfs": true,
}

func skipMount(mountpoint string) bool {
	return strings.HasPrefix(mountpoint, "/snap/") ||
		strings.HasPrefix(mountpoint, "/boot/efi") ||
		strings.HasPrefix(mountpoint, "/boot/firmware")
}

// DiskCollector collects disk usage and I/O metrics.
type DiskCollector struct {
	mu       sync.Mutex
	prevIO   map[string]disk.IOCountersStat
	prevTime time.Time
}

func NewDiskCollector() *DiskCollector {
	return &DiskCollector{}
}

func (c *DiskCollector) Name() string { return "disk" }

func (c *DiskCollector) Collect(ctx context.Context) ([]Metric, error) {
	var metrics []Metric

	// List partitions, filter virtual filesystems
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, err
	}

	for _, p := range partitions {
		if virtualFS[p.Fstype] || skipMount(p.Mountpoint) {
			continue
		}

		usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil {
			continue
		}

		mount := p.Mountpoint
		metrics = append(metrics,
			Metric{Name: DiskMetric(mount, "usage_pct"), Value: usage.UsedPercent},
			Metric{Name: DiskMetric(mount, "total_bytes"), Value: float64(usage.Total)},
			Metric{Name: DiskMetric(mount, "used_bytes"), Value: float64(usage.Used)},
			Metric{Name: DiskMetric(mount, "avail_bytes"), Value: float64(usage.Free)},
		)
	}

	// IO counters (compute delta)
	ioCounters, err := disk.IOCountersWithContext(ctx)
	if err == nil {
		now := time.Now()
		c.mu.Lock()
		if c.prevIO != nil && !c.prevTime.IsZero() {
			elapsed := now.Sub(c.prevTime).Seconds()
			if elapsed > 0 {
				for name, curr := range ioCounters {
					if strings.HasPrefix(name, "loop") {
						continue
					}
					if prev, ok := c.prevIO[name]; ok {
						readIOPS := float64(curr.ReadCount-prev.ReadCount) / elapsed
						writeIOPS := float64(curr.WriteCount-prev.WriteCount) / elapsed
						readBytes := float64(curr.ReadBytes-prev.ReadBytes) / elapsed
						writeBytes := float64(curr.WriteBytes-prev.WriteBytes) / elapsed

						// Use device name as mount qualifier for IOPS
						metrics = append(metrics,
							Metric{Name: DiskMetric(name, "read_iops"), Value: readIOPS},
							Metric{Name: DiskMetric(name, "write_iops"), Value: writeIOPS},
							Metric{Name: DiskMetric(name, "read_bytes"), Value: readBytes},
							Metric{Name: DiskMetric(name, "write_bytes"), Value: writeBytes},
						)
					}
				}
			}
		}
		c.prevIO = ioCounters
		c.prevTime = now
		c.mu.Unlock()
	}

	return metrics, nil
}
