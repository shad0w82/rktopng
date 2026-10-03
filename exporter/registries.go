package main

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"rktopng/collectors"
)

// gatherers hands each endpoint the registry that holds only what it needs.
// Every Gather runs every registered collector, so serving the live stream from
// a registry that also contains the process scan and the static identity
// collectors would repeat that work every second for nothing.
type gatherers struct {
	all  prometheus.Gatherer // /metrics and ?view=all: everything
	live prometheus.Gatherer // /api/stream: what changes, without processes or identity
	info prometheus.Gatherer // /api/info: board and disk identity
}

// of returns the gatherer to use for a view.
func (g gatherers) of(v view) prometheus.Gatherer {
	switch v {
	case viewLive:
		return g.live
	case viewInfo:
		return g.info
	}
	return g.all
}

// newRegistries builds the three registries. The collectors are shared between
// them, so they hold a single state (previous CPU sample, SMART cache, ...).
//
// The live collectors are collected at most once per `share`: clients that gather
// at the same moment get the same reading instead of stealing each other's
// interval (see collectors.Cached). Pick it a bit under the stream interval.
func newRegistries(proc *collectors.ProcessCollector, smart *collectors.SMARTCollector, share time.Duration) (all *prometheus.Registry, g gatherers) {
	cached := func(c prometheus.Collector) prometheus.Collector { return collectors.Cached(c, share) }
	var (
		socInfo  = collectors.NewInfoCollector()
		diskInfo = collectors.NewDiskInfoCollector()
		// Collectors whose output changes while the exporter runs.
		live = []prometheus.Collector{
			cached(collectors.NewCPUCollector()),
			cached(collectors.NewThermalCollector()),
			cached(collectors.NewGPUCollector()),
			cached(collectors.NewNPUCollector()),
			cached(collectors.NewRGACollector()),
			cached(collectors.NewVPUCollector()),
			cached(collectors.NewDDRCollector()),
			cached(collectors.NewMemoryCollector()),
			cached(collectors.NewPowerCollector()),
			cached(collectors.NewDiskIOCollector()),
			cached(collectors.NewDiskTempCollector(smart)),
			smart, // already a cache of a background poll; its smart_info (static) is filtered out of the live view
			cached(collectors.NewFilesystemCollector()),
			cached(collectors.NewZFSCollector()),
			cached(collectors.NewTypeCCollector()),
			cached(collectors.NewSystemCollector()),
		}
	)

	build := func(cs ...prometheus.Collector) *prometheus.Registry {
		r := prometheus.NewRegistry()
		r.MustRegister(cs...)
		return r
	}

	all = build(append(append([]prometheus.Collector{socInfo, diskInfo}, live...), proc)...)
	liveReg := build(live...)
	infoReg := build(socInfo, diskInfo, smart) // smart carries smart_info (disk identity)
	return all, gatherers{all: all, live: liveReg, info: infoReg}
}
