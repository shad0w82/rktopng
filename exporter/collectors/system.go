package collectors

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// SystemCollector exposes uptime, load average, network throughput and TCP
// connection count. (Filesystem usage lives in FilesystemCollector.)
type SystemCollector struct {
	uptime  *prometheus.Desc
	load    *prometheus.Desc
	netRx   *prometheus.Desc
	netTx   *prometheus.Desc
	netInfo *prometheus.Desc
	netLink *prometheus.Desc
	tcpConn *prometheus.Desc

	mu       sync.Mutex
	prevNet  map[string][2]uint64 // iface -> {rx, tx}
	prevTime time.Time
}

func NewSystemCollector() *SystemCollector {
	return &SystemCollector{
		uptime: prometheus.NewDesc(Namespace+"_uptime_seconds",
			"System uptime in seconds.", nil, nil),
		load: prometheus.NewDesc(Namespace+"_load_average",
			"System load average.", []string{"period"}, nil),
		netRx: prometheus.NewDesc(Namespace+"_network_receive_bytes_per_second",
			"Network receive throughput in bytes/sec.", []string{"iface"}, nil),
		netTx: prometheus.NewDesc(Namespace+"_network_transmit_bytes_per_second",
			"Network transmit throughput in bytes/sec.", []string{"iface"}, nil),
		netInfo: prometheus.NewDesc(Namespace+"_network_info",
			"Network interface kind (ethernet|wifi|vpn|other), value is always 1.", []string{"iface", "kind"}, nil),
		netLink: prometheus.NewDesc(Namespace+"_network_speed_mbps",
			"Negotiated link speed of a wired interface in Mbit/s (only while the link is up).", []string{"iface"}, nil),
		tcpConn: prometheus.NewDesc(Namespace+"_tcp_connections",
			"Number of TCP connections (IPv4 + IPv6).", nil, nil),
		prevNet: map[string][2]uint64{},
	}
}

func (c *SystemCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.uptime
	ch <- c.load
	ch <- c.netRx
	ch <- c.netTx
	ch <- c.netInfo
	ch <- c.netLink
	ch <- c.tcpConn
}

func (c *SystemCollector) Collect(ch chan<- prometheus.Metric) {
	c.collectUptime(ch)
	c.collectLoad(ch)
	c.collectNet(ch)
	c.collectTCP(ch)
}

func (c *SystemCollector) collectUptime(ch chan<- prometheus.Metric) {
	if s, err := readTrim(Proc("uptime")); err == nil {
		if f := fields(s); len(f) > 0 {
			if v, err := strconv.ParseFloat(f[0], 64); err == nil {
				ch <- prometheus.MustNewConstMetric(c.uptime, prometheus.GaugeValue, v)
			}
		}
	}
}

func (c *SystemCollector) collectLoad(ch chan<- prometheus.Metric) {
	s, err := readTrim(Proc("loadavg"))
	if err != nil {
		return
	}
	f := fields(s)
	if len(f) < 3 {
		return
	}
	for i, period := range []string{"1m", "5m", "15m"} {
		if v, err := strconv.ParseFloat(f[i], 64); err == nil {
			ch <- prometheus.MustNewConstMetric(c.load, prometheus.GaugeValue, v, period)
		}
	}
}

func (c *SystemCollector) collectNet(ch chan<- prometheus.Metric) {
	ifaces, err := filepath.Glob(Sys("class/net/*"))
	if err != nil {
		return
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	dt := now.Sub(c.prevTime).Seconds()
	first := c.prevTime.IsZero()

	for _, ip := range ifaces {
		iface := filepath.Base(ip)
		if iface == "lo" || strings.HasPrefix(iface, "veth") ||
			strings.HasPrefix(iface, "docker") || strings.HasPrefix(iface, "br-") {
			continue
		}
		rx, e1 := readUint(filepath.Join(ip, "statistics/rx_bytes"))
		tx, e2 := readUint(filepath.Join(ip, "statistics/tx_bytes"))
		if e1 != nil || e2 != nil {
			continue
		}
		ch <- prometheus.MustNewConstMetric(c.netInfo, prometheus.GaugeValue, 1, iface, netKind(ip))
		if mbps, err := readUint(filepath.Join(ip, "speed")); err == nil && mbps > 0 {
			ch <- prometheus.MustNewConstMetric(c.netLink, prometheus.GaugeValue, float64(mbps), iface)
		}
		if !first && dt > 0 {
			if p, ok := c.prevNet[iface]; ok {
				ch <- prometheus.MustNewConstMetric(c.netRx, prometheus.GaugeValue,
					float64(rx-p[0])/dt, iface)
				ch <- prometheus.MustNewConstMetric(c.netTx, prometheus.GaugeValue,
					float64(tx-p[1])/dt, iface)
			}
		}
		c.prevNet[iface] = [2]uint64{rx, tx}
	}
	c.prevTime = now
}

// collectTCP counts TCP sockets. /proc/net/sockstat answers in O(1), whereas
// reading /proc/net/tcp makes the kernel walk the whole socket hash table on
// every read; the full table is only the fallback.
func (c *SystemCollector) collectTCP(ch chan<- prometheus.Metric) {
	count, ok := tcpSockets()
	if !ok {
		for _, p := range []string{Proc("net/tcp"), Proc("net/tcp6")} {
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			if lines := strings.Count(string(data), "\n"); lines > 0 {
				count += float64(lines - 1) // minus header
			}
		}
	}
	ch <- prometheus.MustNewConstMetric(c.tcpConn, prometheus.GaugeValue, count)
}

// tcpSockets sums IPv4 and IPv6 TCP sockets in use plus those in TIME_WAIT
// (which the kernel counts once, on the IPv4 line).
func tcpSockets() (float64, bool) {
	var total float64
	found := false
	if s, err := readTrim(Proc("net/sockstat")); err == nil {
		inuse, tw := parseSockstat(s, "TCP")
		total += inuse + tw
		found = true
	}
	if s, err := readTrim(Proc("net/sockstat6")); err == nil {
		inuse, _ := parseSockstat(s, "TCP6")
		total += inuse
		found = true
	}
	return total, found
}

// parseSockstat reads the "inuse" and "tw" counters of a protocol line such as
// "TCP: inuse 14 orphan 0 tw 5 alloc 20 mem 3".
func parseSockstat(data, proto string) (inuse, tw float64) {
	for _, line := range strings.Split(data, "\n") {
		f := fields(line)
		if len(f) < 3 || f[0] != proto+":" {
			continue
		}
		for i := 1; i+1 < len(f); i += 2 {
			v, err := strconv.ParseFloat(f[i+1], 64)
			if err != nil {
				continue
			}
			switch f[i] {
			case "inuse":
				inuse = v
			case "tw":
				tw = v
			}
		}
	}
	return inuse, tw
}

// netKind says what an interface is, from what sysfs knows about it, so the UI does
// not have to guess from the name: a wireless one has a "wireless" (or phy80211)
// directory; tunnels (tun/tap, WireGuard, Tailscale) have no link layer
// (ARPHRD_NONE, type 65534) or a tun_flags file; an Ethernet one has hardware behind it
// (a "device" link). Anything else (bridges, dummies…) is "other".
func netKind(ifaceDir string) string {
	if exists(filepath.Join(ifaceDir, "wireless")) || exists(filepath.Join(ifaceDir, "phy80211")) {
		return "wifi"
	}
	t, _ := readTrim(filepath.Join(ifaceDir, "type"))
	if t == "65534" || exists(filepath.Join(ifaceDir, "tun_flags")) {
		return "vpn"
	}
	if t == "1" && exists(filepath.Join(ifaceDir, "device")) {
		return "ethernet"
	}
	return "other"
}
