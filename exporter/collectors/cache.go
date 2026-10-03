package collectors

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// cachedCollector runs the collector it wraps at most once per ttl and replays
// that result to everyone who asks in between.
//
// Why: CPU usage, disk throughput and network rate are computed from the difference
// between two consecutive reads. When several clients (browser tabs, a Prometheus
// scrape) gather independently, each one steals the interval of the others: a read
// a few milliseconds after another sees a zero-length window, so the rate is missing
// (or, 10 ms later, wildly quantised). Sharing one reading per ttl keeps every window
// at least ttl long, and makes the cost independent of the number of clients.
type cachedCollector struct {
	inner prometheus.Collector
	ttl   time.Duration

	mu      sync.Mutex
	at      time.Time
	metrics []prometheus.Metric
}

// Cached wraps c so that it is collected at most once per ttl. The same wrapper
// can be registered in several registries: they then share the one reading.
func Cached(c prometheus.Collector, ttl time.Duration) prometheus.Collector {
	return &cachedCollector{inner: c, ttl: ttl}
}

func (c *cachedCollector) Describe(ch chan<- *prometheus.Desc) { c.inner.Describe(ch) }

func (c *cachedCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.at.IsZero() || time.Since(c.at) >= c.ttl {
		c.metrics = c.metrics[:0]
		buf := make(chan prometheus.Metric, 64)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for m := range buf {
				c.metrics = append(c.metrics, m)
			}
		}()
		c.inner.Collect(buf)
		close(buf)
		<-done
		c.at = time.Now()
	}
	for _, m := range c.metrics {
		ch <- m
	}
}
