package collectors

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// DiskIOCollector exposes per-device disk throughput and busy percentage,
// computed from /proc/diskstats deltas. Only whole disks are reported (entries
// present in /sys/block), skipping loop/ram/zram devices.
type DiskIOCollector struct {
	read  *prometheus.Desc
	write *prometheus.Desc
	busy  *prometheus.Desc

	mu       sync.Mutex
	prev     map[string][3]uint64 // device -> {readSectors, writeSectors, ioMillis}
	prevTime time.Time
}

func NewDiskIOCollector() *DiskIOCollector {
	return &DiskIOCollector{
		read: prometheus.NewDesc(Namespace+"_disk_read_bytes_per_second",
			"Disk read throughput in bytes/sec.", []string{"device"}, nil),
		write: prometheus.NewDesc(Namespace+"_disk_write_bytes_per_second",
			"Disk write throughput in bytes/sec.", []string{"device"}, nil),
		busy: prometheus.NewDesc(Namespace+"_disk_busy_percent",
			"Percentage of time the disk was doing I/O.", []string{"device"}, nil),
		prev: map[string][3]uint64{},
	}
}

func (c *DiskIOCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.read
	ch <- c.write
	ch <- c.busy
}

func (c *DiskIOCollector) Collect(ch chan<- prometheus.Metric) {
	data, err := os.ReadFile(Proc("diskstats"))
	if err != nil {
		return
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	dt := now.Sub(c.prevTime).Seconds()
	first := c.prevTime.IsZero()

	for _, line := range strings.Split(string(data), "\n") {
		f := fields(line)
		if len(f) < 14 {
			continue
		}
		name := f[2]
		if !isWholeDisk(name) {
			continue
		}
		rs, _ := strconv.ParseUint(f[5], 10, 64)  // sectors read
		ws, _ := strconv.ParseUint(f[9], 10, 64)  // sectors written
		io, _ := strconv.ParseUint(f[12], 10, 64) // ms spent doing I/O

		if !first && dt > 0 {
			if p, ok := c.prev[name]; ok {
				// Linux disk sectors are 512 bytes.
				ch <- prometheus.MustNewConstMetric(c.read, prometheus.GaugeValue,
					float64(rs-p[0])*512/dt, name)
				ch <- prometheus.MustNewConstMetric(c.write, prometheus.GaugeValue,
					float64(ws-p[1])*512/dt, name)
				busy := float64(io-p[2]) / (dt * 1000.0) * 100.0
				if busy > 100 {
					busy = 100
				}
				ch <- prometheus.MustNewConstMetric(c.busy, prometheus.GaugeValue, busy, name)
			}
		}
		c.prev[name] = [3]uint64{rs, ws, io}
	}
	c.prevTime = now
}
