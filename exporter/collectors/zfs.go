package collectors

import (
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"
)

// ZFSCollector exposes the health of every imported ZFS pool, read from
// /proc/spl/kstat/zfs/<pool>/state (ONLINE, DEGRADED, FAULTED, ...). Readable
// without root and without the zfs userland tools. Emits nothing on systems
// without ZFS.
type ZFSCollector struct {
	online *prometheus.Desc
	state  *prometheus.Desc
}

func NewZFSCollector() *ZFSCollector {
	return &ZFSCollector{
		online: prometheus.NewDesc(Namespace+"_zfs_pool_online",
			"1 if the ZFS pool is ONLINE, 0 otherwise (DEGRADED, FAULTED, ...).", []string{"pool"}, nil),
		state: prometheus.NewDesc(Namespace+"_zfs_pool_state",
			"ZFS pool state (value is always 1; the state is in the label).", []string{"pool", "state"}, nil),
	}
}

func (c *ZFSCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.online
	ch <- c.state
}

func (c *ZFSCollector) Collect(ch chan<- prometheus.Metric) {
	paths, _ := filepath.Glob(Proc("spl/kstat/zfs/*/state"))
	for _, p := range paths {
		state, err := readTrim(p)
		if err != nil || state == "" {
			continue
		}
		pool := filepath.Base(filepath.Dir(p))
		up := 0.0
		if state == "ONLINE" {
			up = 1
		}
		ch <- prometheus.MustNewConstMetric(c.online, prometheus.GaugeValue, up, pool)
		ch <- prometheus.MustNewConstMetric(c.state, prometheus.GaugeValue, 1, pool, state)
	}
}
