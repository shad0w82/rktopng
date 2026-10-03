package collectors

import (
	"fmt"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// A process list served by /api/processes must not be minutes old when nobody is
// streaming: a stale cache is re-read, a fresh one is reused.
func TestProcessListRefreshesStaleCache(t *testing.T) {
	proc, _ := fakeRoots(t)
	write(t, filepath.Join(proc, "meminfo"), "MemTotal:       16000000 kB\n")
	addProc := func(pid int) {
		write(t, filepath.Join(proc, strconv.Itoa(pid), "stat"),
			fmt.Sprintf("%d (p%d) S 1 1 1 0 -1 0 0 0 0 0 5 5 0 0 20 0 1 0 100 1000 10 0\n", pid, pid))
	}
	addProc(1)

	c := NewProcessCollector()
	if _, total := c.List("pid", 10, false); total != 1 {
		t.Fatalf("first read: total = %d, want 1", total)
	}

	addProc(2)
	if _, total := c.List("pid", 10, false); total != 1 {
		t.Errorf("a fresh cache must be reused (total = %d, want 1)", total)
	}

	c.mu.Lock()
	c.prevTime = time.Now().Add(-time.Minute)
	c.mu.Unlock()
	procs, total := c.List("pid", 10, false)
	if total != 2 || len(procs) != 2 {
		t.Errorf("a stale cache must be re-read: total = %d, rows = %d, want 2", total, len(procs))
	}
}

func fakeProcs(t *testing.T, n int) *ProcessCollector {
	t.Helper()
	proc, _ := fakeRoots(t)
	write(t, filepath.Join(proc, "meminfo"), "MemTotal:       16000000 kB\n")
	for pid := 1; pid <= n; pid++ {
		// every process has the same CPU time and a different resident size
		write(t, filepath.Join(proc, strconv.Itoa(pid), "stat"),
			fmt.Sprintf("%d (p%d) S 1 1 1 0 -1 0 0 0 0 0 5 5 0 0 20 0 1 0 100 1000 %d 0\n", pid, pid, pid*10))
	}
	return NewProcessCollector()
}

func pids(ps []ProcInfo) []int {
	out := make([]int, len(ps))
	for i, p := range ps {
		out[i] = p.Pid
	}
	return out
}

func TestProcessListReverseReturnsTheOtherEnd(t *testing.T) {
	c := fakeProcs(t, 6)
	asc, _ := c.List("pid", 2, false)
	if got := pids(asc); got[0] != 1 || got[1] != 2 {
		t.Fatalf("pid ascending = %v", got)
	}
	// reversing the page would give [2 1]; the other end of the whole list is [6 5]
	rev, total := c.List("pid", 2, true)
	if got := pids(rev); got[0] != 6 || got[1] != 5 || total != 6 {
		t.Errorf("pid reversed = %v (total %d), want [6 5] (total 6)", got, total)
	}
	mem, _ := c.List("mem", 3, false) // biggest first by default
	if got := pids(mem); got[0] != 6 {
		t.Errorf("mem = %v, want the largest first", got)
	}
	least, _ := c.List("mem", 1, true)
	if got := pids(least); got[0] != 1 {
		t.Errorf("mem reversed = %v, want the smallest", got)
	}
}

// Hundreds of processes sit at 0% CPU: with equal keys the order must not change between calls.
func TestProcessListTiesKeepASteadyOrder(t *testing.T) {
	c := fakeProcs(t, 30)
	c.List("pid", 1, false) // read /proc once so that there is a cached list to adjust
	for i := range c.last { // make every row tie on CPU and memory
		c.last[i].MemBytes = 1
	}
	c.prevTime = time.Now()
	a, _ := c.List("cpu", 30, false)
	b, _ := c.List("cpu", 30, false)
	if fmt.Sprint(pids(a)) != fmt.Sprint(pids(b)) {
		t.Errorf("order changed between calls:\n%v\n%v", pids(a), pids(b))
	}
	for i := 1; i < len(a); i++ {
		if a[i-1].Pid > a[i].Pid {
			t.Fatalf("ties are not ordered by pid: %v", pids(a))
		}
	}
}
