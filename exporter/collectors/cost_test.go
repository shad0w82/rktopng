package collectors

import (
	"path/filepath"
	"testing"
	"time"
)

// These tests cover the cheap read paths that replaced expensive ones (see
// docs/FUNZIONAMENTO.md, "Cost of a gather"): each must give the same answer as
// the slow source it stands in for, and keep the slow source as a fallback.

func TestParseStat(t *testing.T) {
	// pid (comm) state ppid pgrp session tty tpgid flags minflt cminflt majflt cmajflt utime stime ... threads ... rss
	stat := "42 (my (odd) proc) R 1 1 1 0 -1 0 0 0 0 0 7 3 0 0 20 0 9 0 100 1000 25 0\n"
	p, ok := parseStat([]byte(stat), 42, 4096)
	if !ok {
		t.Fatal("parseStat rejected a valid line")
	}
	if p.Name != "my (odd) proc" || p.State != "R" || p.Threads != 9 || p.MemBytes != 25*4096 || p.jiffies != 10 {
		t.Errorf("parsed %+v", p)
	}
	for _, bad := range []string{"", "42 no parens S 1", "42 (short) S 1 2 3"} {
		if _, ok := parseStat([]byte(bad), 1, 4096); ok {
			t.Errorf("parseStat accepted %q", bad)
		}
	}
}

func TestParseSockstat(t *testing.T) {
	in := "sockets: used 120\nTCP: inuse 14 orphan 1 tw 5 alloc 20 mem 3\nUDP: inuse 4 mem 1\n"
	if inuse, tw := parseSockstat(in, "TCP"); inuse != 14 || tw != 5 {
		t.Errorf("TCP = %v/%v, want 14/5", inuse, tw)
	}
	if inuse, _ := parseSockstat("TCP6: inuse 2\n", "TCP6"); inuse != 2 {
		t.Errorf("TCP6 inuse = %v, want 2", inuse)
	}
	if inuse, tw := parseSockstat(in, "TCP6"); inuse != 0 || tw != 0 {
		t.Errorf("a missing protocol must read as 0, got %v/%v", inuse, tw)
	}
}

func tcpConnections(t *testing.T) float64 {
	t.Helper()
	m := gather(t, NewSystemCollector())
	ss := m["rk3588_tcp_connections"]
	if len(ss) != 1 {
		t.Fatalf("tcp_connections = %v", ss)
	}
	return ss[0].value
}

func TestTCPConnectionsFromSockstat(t *testing.T) {
	proc, _ := fakeRoots(t)
	write(t, filepath.Join(proc, "net/sockstat"), "sockets: used 90\nTCP: inuse 14 orphan 0 tw 5 alloc 20 mem 3\n")
	write(t, filepath.Join(proc, "net/sockstat6"), "TCP6: inuse 3\n")
	// A stale full table that must NOT be used when sockstat exists.
	write(t, filepath.Join(proc, "net/tcp"), "header\nA\nB\n")
	if got := tcpConnections(t); got != 22 { // 14 + 5 time-wait + 3 IPv6
		t.Errorf("tcp connections = %v, want 22", got)
	}
}

func TestTCPConnectionsFallbackToFullTable(t *testing.T) {
	proc, _ := fakeRoots(t)
	write(t, filepath.Join(proc, "net/tcp"), "header\nA\nB\n")
	write(t, filepath.Join(proc, "net/tcp6"), "header\nC\n")
	if got := tcpConnections(t); got != 3 {
		t.Errorf("tcp connections = %v, want 3", got)
	}
}

func TestRGAFrequencyFromPerClockFiles(t *testing.T) {
	_, sys := fakeRoots(t)
	write(t, filepath.Join(sys, "kernel/debug/clk/clk_rga3_0_core/clk_rate"), "750000000\n")
	write(t, filepath.Join(sys, "kernel/debug/clk/clk_rga2_core/clk_rate"), "300000000\n")
	// The big dump must not be consulted when the small files exist.
	write(t, filepath.Join(sys, "kernel/debug/clk/clk_summary"), "clk_rga3_1_core 1 1 0 999000000 0 0 50000\n")

	f := gather(t, NewRGACollector())["rk3588_rga_freq_mhz"]
	if got := find(t, f, "scheduler", "rga3_0").value; got != 750 {
		t.Errorf("rga3_0 = %v MHz, want 750", got)
	}
	if got := find(t, f, "scheduler", "rga2_2").value; got != 300 {
		t.Errorf("rga2_2 = %v MHz, want 300", got)
	}
	if has(f, "scheduler", "rga3_1") {
		t.Error("clk_summary was read although per-clock files exist")
	}
}

func TestRGAFrequencyFallsBackToClockSummary(t *testing.T) {
	_, sys := fakeRoots(t)
	write(t, filepath.Join(sys, "kernel/debug/clk/clk_summary"),
		"   clock    enable prepare protect  rate  accuracy phase duty\n"+
			"clk_rga3_1_core 1 1 0 600000000 0 0 50000\n")
	f := gather(t, NewRGACollector())["rk3588_rga_freq_mhz"]
	if got := find(t, f, "scheduler", "rga3_1").value; got != 600 {
		t.Errorf("rga3_1 = %v MHz, want 600", got)
	}
}

// NVMe temperatures cost a SMART command per read, so two collectors asking
// within a moment must share one reading.
func TestNVMeSensorsAreCached(t *testing.T) {
	_, sys := fakeRoots(t)
	d := filepath.Join(sys, "class/hwmon/hwmon0")
	write(t, filepath.Join(d, "name"), "nvme\n")
	write(t, filepath.Join(d, "temp1_input"), "40000\n")
	if s := nvmeSensors(); len(s) != 1 || s[0].celsius != 40 {
		t.Fatalf("first read = %+v", s)
	}

	write(t, filepath.Join(d, "temp1_input"), "55000\n")
	if s := nvmeSensors(); s[0].celsius != 40 {
		t.Errorf("a fresh reading must be reused, got %v", s[0].celsius)
	}

	nvmeCache.mu.Lock()
	nvmeCache.at = time.Now().Add(-2 * nvmeCacheTTL)
	nvmeCache.mu.Unlock()
	if s := nvmeSensors(); s[0].celsius != 55 {
		t.Errorf("an expired reading must be refreshed, got %v", s[0].celsius)
	}
}

func TestNetworkInterfaceKindAndSpeed(t *testing.T) {
	_, sys := fakeRoots(t)
	iface := func(name, typ string, files map[string]string) {
		d := filepath.Join(sys, "class/net", name)
		write(t, filepath.Join(d, "statistics/rx_bytes"), "1000\n")
		write(t, filepath.Join(d, "statistics/tx_bytes"), "2000\n")
		write(t, filepath.Join(d, "type"), typ+"\n")
		for f, v := range files {
			write(t, filepath.Join(d, f), v)
		}
	}
	iface("eth0", "1", map[string]string{"device/uevent": "x", "speed": "1000\n"})
	iface("wlan0", "1", map[string]string{"device/uevent": "x", "wireless/status": "0\n", "speed": "-1\n"})
	iface("tailscale0", "65534", map[string]string{"tun_flags": "0x1002\n", "speed": "-1\n"})
	iface("dummy0", "1", nil)

	m := gather(t, NewSystemCollector())
	want := map[string]string{"eth0": "ethernet", "wlan0": "wifi", "tailscale0": "vpn", "dummy0": "other"}
	for name, kind := range want {
		if got := find(t, m["rk3588_network_info"], "iface", name).labels["kind"]; got != kind {
			t.Errorf("%s kind = %q, want %q", name, got, kind)
		}
	}
	// only a link that is up reports a speed (-1 means unknown)
	speeds := m["rk3588_network_speed_mbps"]
	if len(speeds) != 1 || find(t, speeds, "iface", "eth0").value != 1000 {
		t.Errorf("speeds = %v, want only eth0 at 1000", speeds)
	}
}
