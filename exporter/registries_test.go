package main

import (
	"os"
	"path/filepath"
	"testing"

	"rktopng/collectors"
)

// held lists the metric families a registry produces, whatever the view.
func held(t *testing.T, gs gatherers, v view) map[string]bool {
	t.Helper()
	return names(gatherSnapshot(gs.of(v), viewAll))
}

// The live registry feeds the 1 Hz stream. It must not contain the collectors
// whose output the live view throws away (process scan, static identity),
// otherwise every push pays for them.
func TestRegistriesSplitWorkByEndpoint(t *testing.T) {
	fakeProc(t, 3)
	if err := os.WriteFile(filepath.Join(collectors.ProcPath, "uptime"), []byte("100.0 90.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	all, gs := newRegistries(collectors.NewProcessCollector(), collectors.NewSMARTCollector(), 0)
	if all == nil {
		t.Fatal("no registry for /metrics")
	}

	every := held(t, gs, viewAll)
	for _, n := range []string{"rk3588_process_info", "rk3588_soc_info", "rk3588_uptime_seconds"} {
		if !every[n] {
			t.Errorf("/metrics must carry %s", n)
		}
	}

	live := held(t, gs, viewLive)
	if !live["rk3588_uptime_seconds"] {
		t.Error("the live registry lacks a live metric")
	}
	for _, n := range []string{"rk3588_process_info", "rk3588_process_cpu_percent", "rk3588_soc_info", "rk3588_disk_info"} {
		if live[n] {
			t.Errorf("the live registry must not run the collector of %s", n)
		}
	}

	info := held(t, gs, viewInfo)
	if !info["rk3588_soc_info"] {
		t.Error("the info registry lacks the board identity")
	}
	for _, n := range []string{"rk3588_uptime_seconds", "rk3588_process_info"} {
		if info[n] {
			t.Errorf("the info registry must hold only identity collectors, found %s", n)
		}
	}
}

func TestGatherersOf(t *testing.T) {
	_, gs := newRegistries(collectors.NewProcessCollector(), collectors.NewSMARTCollector(), 0)
	if gs.of(viewAll) != gs.all || gs.of(viewLive) != gs.live || gs.of(viewInfo) != gs.info {
		t.Error("of() returns the wrong registry for a view")
	}
}
