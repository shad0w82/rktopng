// Values the UI computes from the exporter's metrics (docs/FUNZIONAMENTO.md §5).
// Pure functions of snapshots, so they are easy to test; the stores call them.

import type { Snapshot } from '../api/types'
import type { Disk } from './inventory'
import { M, label, samples, val } from './metrics'
import { ZONES } from './names'

/** Mean usage of the CPU cores, %. */
export function cpuAverage(s: Snapshot | null): number | undefined {
  const xs = samples(s, M.cpuUsage)
  if (xs.length === 0) return undefined
  return xs.reduce((a, x) => a + x.value, 0) / xs.length
}

export interface MemBreakdown {
  total: number
  /** Memory that cannot be reclaimed: total − available. This is the "RAM %" the gauges show. */
  used: number
  /** Page cache + buffers that count as available. */
  cache: number
  /** The rest, so that used + cache + free = total. */
  free: number
  available: number
}

export function memBreakdown(s: Snapshot | null): MemBreakdown | undefined {
  const total = val(s, M.memory, { type: 'total' })
  const available = val(s, M.memory, { type: 'available' })
  if (!total || available === undefined) return undefined
  const cached = val(s, M.memory, { type: 'cached' }) ?? 0
  const buffers = val(s, M.memory, { type: 'buffers' }) ?? 0
  const used = total - available
  const cache = Math.min(cached + buffers, available)
  return { total, used, cache, free: total - used - cache, available }
}

/** RAM used, %: (total − available) / total, like btop/htop (not total − free, which counts the cache). */
export function ramUsedPct(s: Snapshot | null): number | undefined {
  const m = memBreakdown(s)
  return m ? (m.used / m.total) * 100 : undefined
}

export interface Usage {
  total: number
  used: number
  pct: number
}

export function swapUsage(s: Snapshot | null): Usage | undefined {
  const total = val(s, M.swap, { type: 'total' })
  const used = val(s, M.swap, { type: 'used' })
  if (total === undefined || used === undefined) return undefined
  return { total, used, pct: total > 0 ? (used / total) * 100 : 0 }
}

/** CMA (contiguous memory pool used by the media blocks); absent on kernels without it. */
export function cmaUsage(s: Snapshot | null): Usage | undefined {
  const total = val(s, M.memory, { type: 'cma_total' })
  const used = val(s, M.memory, { type: 'cma_used' })
  if (!total || used === undefined) return undefined
  return { total, used, pct: (used / total) * 100 }
}

/** Fan duty: the exporter reports PWM 0–255. */
export function fanLevel(s: Snapshot | null): { raw: number; pct: number } | undefined {
  const raw = val(s, M.fanPwm)
  return raw === undefined ? undefined : { raw, pct: (raw / 255) * 100 }
}

export interface Volume {
  mount: string
  fstype: string
  /** Device, or ZFS pool/dataset. */
  source: string
  size: number
  used: number
  avail: number
  /** As `df` shows it: used / (used + avail), not used / size (reserved blocks are excluded). */
  pct: number
}

export function volumes(s: Snapshot | null): Volume[] {
  return samples(s, M.fsSize)
    .map((sz) => {
      const mount = label(sz, 'mount')
      const used = val(s, M.fsUsed, { mount }) ?? 0
      const avail = val(s, M.fsAvail, { mount }) ?? 0
      const denom = used + avail
      return {
        mount,
        fstype: label(sz, 'fstype'),
        source: label(sz, 'source'),
        size: sz.value,
        used,
        avail,
        pct: denom > 0 ? (used / denom) * 100 : 0,
      }
    })
    .sort((a, b) => (a.mount === '/' ? -1 : b.mount === '/' ? 1 : a.mount.localeCompare(b.mount)))
}

/** ZFS pools that are ONLINE out of all pools ("2/2 online"); undefined without ZFS. */
export function zfsSummary(s: Snapshot | null): { online: number; total: number } | undefined {
  const pools = samples(s, M.zfsOnline)
  if (pools.length === 0) return undefined
  return { online: pools.filter((p) => p.value === 1).length, total: pools.length }
}

export interface LoadBreakdown {
  user: number
  system: number
  iowait: number
  idle: number
}

/**
 * Where the CPU time went between two snapshots, in %. The exporter gives
 * cumulative seconds per mode (a counter), so a rate needs two readings.
 * user includes nice; system includes irq and softirq.
 */
export function loadBreakdown(prev: Snapshot | null, cur: Snapshot | null): LoadBreakdown | undefined {
  if (!prev || !cur) return undefined
  const modes = (s: Snapshot) => Object.fromEntries(samples(s, M.cpuSeconds).map((x) => [label(x, 'mode'), x.value]))
  const a = modes(prev)
  const b = modes(cur)
  const d = (k: string) => (b[k] ?? 0) - (a[k] ?? 0)
  const user = d('user') + d('nice')
  const system = d('system') + d('irq') + d('softirq')
  const iowait = d('iowait')
  const idle = d('idle')
  const total = user + system + iowait + idle
  if (!(total > 0) || user < 0 || system < 0 || iowait < 0 || idle < 0) return undefined // counters reset
  return {
    user: (user / total) * 100,
    system: (system / total) * 100,
    iowait: (iowait / total) * 100,
    idle: (idle / total) * 100,
  }
}

export interface Throughput {
  read: number
  write: number
}

/**
 * Read and write throughput summed per bus (nvme / sata / emmc…). `busOf` maps a
 * device to its bus (from the disk inventory). These are physical I/O figures.
 */
export function throughputByBus(s: Snapshot | null, busOf: Record<string, string>): Record<string, Throughput> {
  const out: Record<string, Throughput> = {}
  const add = (name: string, dir: keyof Throughput) => {
    for (const x of samples(s, name)) {
      const bus = busOf[label(x, 'device')]
      if (!bus) continue
      out[bus] ??= { read: 0, write: 0 }
      out[bus][dir] += x.value
    }
  }
  add(M.diskRead, 'read')
  add(M.diskWrite, 'write')
  return out
}

export interface DiskTemperature {
  device: string
  bus: string
  sizeBytes: number | undefined
  /** undefined when the disk has no sensor (eMMC) or SMART has not answered yet → "n/a". */
  celsius: number | undefined
}

/** Every known disk (in the inventory's order) with its current temperature, if it has one. */
export function diskTemperatures(disks: Disk[], s: Snapshot | null): DiskTemperature[] {
  return disks.map((d) => ({
    device: d.device,
    bus: d.bus,
    sizeBytes: d.sizeBytes,
    celsius: val(s, M.diskTemp, { device: d.device }),
  }))
}

/** The cpufreq governor ("ondemand", "schedutil"…), shared by all cores. */
export function cpuGovernor(s: Snapshot | null): string | undefined {
  const g = samples(s, M.cpuGovernor)[0]
  return g ? label(g, 'governor') || undefined : undefined
}

export interface ZoneTemperature {
  zone: string
  /** Short name shown on the tile (little, big_0…). */
  label: string
  /** undefined when the kernel does not report that zone. */
  celsius: number | undefined
}

/** The seven RK3588 thermal zones in display order, each with its current temperature if reported. */
export function zoneTemperatures(s: Snapshot | null): ZoneTemperature[] {
  return ZONES.map(({ zone, label }) => ({ zone, label, celsius: val(s, M.temp, { zone }) }))
}
