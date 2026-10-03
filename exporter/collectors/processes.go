package collectors

import (
	"bytes"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ProcInfo is one process, as shown in a btop-style list.
type ProcInfo struct {
	Pid      int     `json:"pid"`
	Name     string  `json:"name"`
	User     string  `json:"user"`
	Cmd      string  `json:"cmd"`
	State    string  `json:"state"`
	Cpu      float64 `json:"cpu"` // percent, 100 = one core
	MemPct   float64 `json:"mem_pct"`
	MemBytes uint64  `json:"mem_bytes"`
	Threads  int     `json:"threads"`
	jiffies  uint64  // internal: utime+stime for the CPU delta
}

// ProcessCollector reads processes from /proc once per gather, caches the full
// list, and (a) emits the top-N by CPU as Prometheus metrics, (b) serves the
// cached list sorted by any column via List(), for the /api/processes endpoint.
type ProcessCollector struct {
	topN     int
	pageSize uint64

	cpuPct  *prometheus.Desc
	memPct  *prometheus.Desc
	memByte *prometheus.Desc
	threads *prometheus.Desc
	info    *prometheus.Desc

	mu        sync.Mutex
	buf       []byte         // reused by readStat so a scan does not allocate per process
	prev      map[int]uint64 // pid -> (utime+stime) jiffies
	prevTime  time.Time
	userCache map[uint32]string
	last      []ProcInfo // cached full list from the most recent read (no Cmd)
}

func NewProcessCollector() *ProcessCollector {
	topN := 20
	if v, err := strconv.Atoi(getenv("RKTOP_PROC_TOP", "20")); err == nil && v > 0 {
		topN = v
	}
	return &ProcessCollector{
		topN:     topN,
		pageSize: uint64(os.Getpagesize()),
		cpuPct: prometheus.NewDesc(Namespace+"_process_cpu_percent",
			"Per-process CPU usage percent (100 = one core).", []string{"pid"}, nil),
		memPct: prometheus.NewDesc(Namespace+"_process_mem_percent",
			"Per-process memory usage percent of total RAM.", []string{"pid"}, nil),
		memByte: prometheus.NewDesc(Namespace+"_process_mem_bytes",
			"Per-process resident memory in bytes.", []string{"pid"}, nil),
		threads: prometheus.NewDesc(Namespace+"_process_threads",
			"Per-process thread count.", []string{"pid"}, nil),
		info: prometheus.NewDesc(Namespace+"_process_info",
			"Per-process metadata (value is always 1).",
			[]string{"pid", "name", "user", "cmd", "state"}, nil),
		buf:       make([]byte, 4096),
		prev:      map[int]uint64{},
		userCache: map[uint32]string{},
	}
}

func (c *ProcessCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.cpuPct
	ch <- c.memPct
	ch <- c.memByte
	ch <- c.threads
	ch <- c.info
}

func (c *ProcessCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	// Same freshness rule as List: a scrape right after a poll reuses the list, so
	// the CPU% window is never shorter than procCacheMaxAge.
	if len(c.last) == 0 || time.Since(c.prevTime) > procCacheMaxAge {
		c.readAllLocked()
	}
	procs := make([]ProcInfo, len(c.last))
	copy(procs, c.last)
	c.mu.Unlock()

	sortProcs(procs, "cpu")
	if len(procs) > c.topN {
		procs = procs[:c.topN]
	}
	for i := range procs {
		procs[i].Cmd = c.cmdline(procs[i].Pid)
		p := procs[i]
		pid := strconv.Itoa(p.Pid)
		ch <- prometheus.MustNewConstMetric(c.cpuPct, prometheus.GaugeValue, p.Cpu, pid)
		ch <- prometheus.MustNewConstMetric(c.memByte, prometheus.GaugeValue, float64(p.MemBytes), pid)
		ch <- prometheus.MustNewConstMetric(c.memPct, prometheus.GaugeValue, p.MemPct, pid)
		ch <- prometheus.MustNewConstMetric(c.threads, prometheus.GaugeValue, float64(p.Threads), pid)
		ch <- prometheus.MustNewConstMetric(c.info, prometheus.GaugeValue, 1,
			pid, p.Name, p.User, p.Cmd, p.State)
	}
}

// procCacheMaxAge is how old the cached process list may get before List re-reads /proc.
// It also bounds how often the scan runs, which keeps the CPU% deltas meaningful:
// the kernel counts CPU time in 10 ms ticks, so over a very short window one tick
// would read as a huge percentage.
const procCacheMaxAge = time.Second

// List returns the cached process list sorted by key (cpu|mem|pid|name|user|
// threads), truncated to limit, with cmdlines filled in for the returned rows,
// plus the total number of processes before truncation (the UI's "N task").
// reverse flips the order *before* truncating, so it returns the other end of the
// whole list (the least busy, the highest pids), not the same rows upside down.
// Used by the /api/processes endpoint.
func (c *ProcessCollector) List(key string, limit int, reverse bool) ([]ProcInfo, int) {
	c.mu.Lock()
	// The cache is normally refreshed by every gather (the live stream ticks every
	// second). Re-read when it is empty (cold start; CPU% will be 0 this once) or
	// stale (nobody is streaming/scraping), so the list is never minutes old.
	if len(c.last) == 0 || time.Since(c.prevTime) > procCacheMaxAge {
		c.readAllLocked()
	}
	out := make([]ProcInfo, len(c.last))
	copy(out, c.last)
	c.mu.Unlock()
	total := len(out)

	sortProcs(out, key)
	if reverse {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	for i := range out {
		out[i].Cmd = c.cmdline(out[i].Pid)
	}
	return out, total
}

// readAllLocked reads every process's cheap fields (name/state/threads/mem/cpu/
// user), updates the CPU-delta state, caches the result, and returns it. The
// caller must hold c.mu. Cmdlines are NOT read here (deferred to the shown rows).
func (c *ProcessCollector) readAllLocked() []ProcInfo {
	dir, err := os.Open(ProcPath)
	if err != nil {
		return c.last
	}
	names, _ := dir.Readdirnames(-1)
	dir.Close()

	now := time.Now()
	dt := now.Sub(c.prevTime).Seconds()
	first := c.prevTime.IsZero()
	totalMem := c.totalMem()
	newPrev := make(map[int]uint64, len(names))
	procs := make([]ProcInfo, 0, len(names))

	for _, name := range names {
		pid, err := strconv.Atoi(name)
		if err != nil {
			continue
		}
		p, ok := c.readStat(pid)
		if !ok {
			continue
		}
		newPrev[pid] = p.jiffies
		if !first && dt > 0 {
			if prevJ, ok := c.prev[pid]; ok && p.jiffies >= prevJ {
				p.Cpu = float64(p.jiffies-prevJ) / clkTck / dt * 100
			}
		}
		if totalMem > 0 {
			p.MemPct = float64(p.MemBytes) / float64(totalMem) * 100
		}
		p.User = c.user(pid)
		procs = append(procs, p)
	}
	c.prev = newPrev
	c.prevTime = now
	c.last = procs
	return procs
}

// sortProcs orders the list by key. Ties fall back to the pid, so that rows that compare
// equal (hundreds of processes sit at 0% CPU) keep the same order on every refresh
// instead of shuffling.
func sortProcs(p []ProcInfo, key string) {
	by := func(less func(a, b *ProcInfo) (lt, eq bool)) {
		sort.Slice(p, func(i, j int) bool {
			if lt, eq := less(&p[i], &p[j]); !eq {
				return lt
			}
			return p[i].Pid < p[j].Pid
		})
	}
	switch key {
	case "mem":
		by(func(a, b *ProcInfo) (bool, bool) { return a.MemBytes > b.MemBytes, a.MemBytes == b.MemBytes })
	case "threads":
		by(func(a, b *ProcInfo) (bool, bool) { return a.Threads > b.Threads, a.Threads == b.Threads })
	case "pid":
		by(func(a, b *ProcInfo) (bool, bool) { return a.Pid < b.Pid, a.Pid == b.Pid })
	case "name":
		by(func(a, b *ProcInfo) (bool, bool) {
			x, y := strings.ToLower(a.Name), strings.ToLower(b.Name)
			return x < y, x == y
		})
	case "user":
		by(func(a, b *ProcInfo) (bool, bool) {
			x, y := strings.ToLower(a.User), strings.ToLower(b.User)
			if x != y {
				return x < y, false
			}
			return a.Cpu > b.Cpu, a.Cpu == b.Cpu
		})
	default: // "cpu"
		by(func(a, b *ProcInfo) (bool, bool) {
			if a.Cpu != b.Cpu {
				return a.Cpu > b.Cpu, false
			}
			return a.MemBytes > b.MemBytes, a.MemBytes == b.MemBytes
		})
	}
}

// readStat reads /proc/<pid>/stat with a single read into the shared buffer
// (the file is a few hundred bytes and procfs returns it whole). A scan reads
// hundreds of these, so it avoids allocating per file. The caller holds c.mu.
func (c *ProcessCollector) readStat(pid int) (ProcInfo, bool) {
	f, err := os.Open(Proc(strconv.Itoa(pid) + "/stat"))
	if err != nil {
		return ProcInfo{}, false
	}
	n, _ := f.Read(c.buf)
	f.Close()
	return parseStat(c.buf[:n], pid, c.pageSize)
}

// parseStat parses the content of /proc/<pid>/stat. comm is wrapped in parens
// and may contain spaces or parens, so the fields after it are split from the
// last ')'. Field N of proc(5) is rest[N-3].
func parseStat(b []byte, pid int, pageSize uint64) (ProcInfo, bool) {
	lp := bytes.IndexByte(b, '(')
	rp := bytes.LastIndexByte(b, ')')
	if lp < 0 || rp < lp {
		return ProcInfo{}, false
	}
	var state, utime, stime, threads, rss []byte
	i := rp + 1
	for idx := 0; idx < 22; idx++ {
		for i < len(b) && (b[i] == ' ' || b[i] == '\n') {
			i++
		}
		if i >= len(b) {
			return ProcInfo{}, false // fewer than 22 fields
		}
		start := i
		for i < len(b) && b[i] != ' ' && b[i] != '\n' {
			i++
		}
		switch idx {
		case 0:
			state = b[start:i]
		case 11: // field 14
			utime = b[start:i]
		case 12: // field 15
			stime = b[start:i]
		case 17: // field 20
			threads = b[start:i]
		case 21: // field 24
			rss = b[start:i]
		}
	}
	return ProcInfo{
		Pid:      pid,
		Name:     string(b[lp+1 : rp]),
		State:    string(state),
		Threads:  int(atou(threads)),
		MemBytes: atou(rss) * pageSize,
		jiffies:  atou(utime) + atou(stime),
	}, true
}

// atou parses a non-negative decimal number; it stops at the first non-digit
// and yields 0 for garbage, which is how a vanished or odd process is treated.
func atou(b []byte) uint64 {
	var n uint64
	for _, ch := range b {
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + uint64(ch-'0')
	}
	return n
}

func (c *ProcessCollector) cmdline(pid int) string {
	data, err := os.ReadFile(Proc(strconv.Itoa(pid) + "/cmdline"))
	if err != nil || len(data) == 0 {
		return ""
	}
	cmd := strings.TrimRight(string(data), "\x00")
	cmd = strings.ReplaceAll(cmd, "\x00", " ")
	if len(cmd) > 200 {
		cmd = cmd[:200]
	}
	return cmd
}

// user resolves the owner uid of a pid to a username (cached), else the uid.
func (c *ProcessCollector) user(pid int) string {
	var st syscall.Stat_t
	if err := syscall.Stat(Proc(strconv.Itoa(pid)), &st); err != nil {
		return ""
	}
	if name, ok := c.userCache[st.Uid]; ok {
		return name
	}
	name := lookupUser(st.Uid)
	c.userCache[st.Uid] = name
	return name
}

func (c *ProcessCollector) totalMem() uint64 {
	data, err := os.ReadFile(Proc("meminfo"))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			f := fields(line)
			if len(f) >= 2 {
				kb, _ := strconv.ParseUint(f[1], 10, 64)
				return kb * 1024
			}
		}
	}
	return 0
}

// lookupUser reads /etc/passwd (RKTOP_PASSWD_PATH) for a uid -> username mapping.
// In a container without the host passwd, the numeric uid is returned.
func lookupUser(uid uint32) string {
	target := strconv.FormatUint(uint64(uid), 10)
	if data, err := os.ReadFile(getenv("RKTOP_PASSWD_PATH", "/etc/passwd")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 && parts[2] == target {
				return parts[0]
			}
		}
	}
	return target
}
