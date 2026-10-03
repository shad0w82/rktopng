// Short in-memory history behind the sparklines and charts (no persistence: a
// reload starts from scratch). One ring buffer per series, filled on every
// live push.

import type { Snapshot } from '../api/types'
import { cpuAverage, fanLevel, loadBreakdown, memBreakdown, throughputByBus } from './derive'
import { M, samples, seriesKey } from './metrics'
import { RingBuffer } from './ringbuffer'

/** Metric families whose every series is recorded as is. */
const TRACKED = [
  M.cpuUsage,
  M.temp,
  M.diskTemp,
  M.gpuLoad,
  M.npuLoad,
  M.rgaLoad,
  M.vpuLoad,
  M.ddrLoad,
  M.fanPwm,
  M.netRx,
  M.netTx,
  M.diskBusy,
  M.diskRead,
  M.diskWrite,
]

/**
 * Series whose chart should start flat at their first value (temperatures, RAM
 * share). Every other series starts from zero, so a load chart grows from empty
 * instead of drawing a fake plateau at whatever the first reading happened to be.
 */
const FLAT_START = new Set<string>([M.temp, M.diskTemp])

/** Keys of the derived series (the raw ones use `seriesKey(name, labels)`). */
export const K = {
  cpuAvg: 'cpu.avg',
  /** Fan duty, % (the raw series is PWM 0-255). */
  fanPct: 'fan.pct',
  /** Share of RAM, %: not reclaimable / reclaimable cache. */
  ramUsed: 'ram.used',
  ramCache: 'ram.cache',
  cpuMode: (mode: 'user' | 'system' | 'iowait' | 'idle') => `cpu.${mode}`,
  /** Summed per-bus throughput, bytes/s. */
  bus: (bus: string, dir: 'read' | 'write') => `bus.${bus}.${dir}`,
}

export class History {
  private buffers = new Map<string, RingBuffer>()

  /** `capacity` samples per series: at one push per second, 40 s of history. */
  constructor(readonly capacity = 40) {}

  /** `initial` is what a brand-new series is pre-filled with. */
  private push(key: string, v: number, initial = 0): void {
    let b = this.buffers.get(key)
    if (!b) {
      b = new RingBuffer(this.capacity, initial)
      this.buffers.set(key, b)
    }
    b.push(v)
  }

  /** Oldest → newest; empty when the series was never seen. */
  get(key: string): number[] {
    return this.buffers.get(key)?.toArray() ?? []
  }

  /**
   * Record one live snapshot. `prev` (the one before) is needed for counters;
   * `busOf` maps each disk to its bus for the summed throughput.
   */
  record(snap: Snapshot, prev: Snapshot | null, busOf: Record<string, string>): void {
    for (const name of TRACKED) {
      for (const s of samples(snap, name)) this.push(seriesKey(name, s.labels), s.value, FLAT_START.has(name) ? s.value : 0)
    }

    const cpu = cpuAverage(snap)
    if (cpu !== undefined) this.push(K.cpuAvg, cpu)

    const fan = fanLevel(snap)
    if (fan) this.push(K.fanPct, fan.pct)

    const mem = memBreakdown(snap)
    if (mem) {
      const used = (mem.used / mem.total) * 100
      const cache = (mem.cache / mem.total) * 100
      this.push(K.ramUsed, used, used)
      this.push(K.ramCache, cache, cache)
    }

    const lb = loadBreakdown(prev, snap)
    // Before the first interval the CPU is shown as all idle.
    if (lb) for (const mode of ['user', 'system', 'iowait', 'idle'] as const) this.push(K.cpuMode(mode), lb[mode], mode === 'idle' ? 100 : 0)

    for (const [bus, t] of Object.entries(throughputByBus(snap, busOf))) {
      this.push(K.bus(bus, 'read'), t.read)
      this.push(K.bus(bus, 'write'), t.write)
    }
  }
}
