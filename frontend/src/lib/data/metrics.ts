import type { Sample, Snapshot } from '../api/types'

/** Names of the exporter metrics the UI uses (all carry the rk3588_ prefix). */
export const M = {
  uptime: 'rk3588_uptime_seconds',
  socInfo: 'rk3588_soc_info',
  temp: 'rk3588_temp_celsius', // label: zone
  cpuUsage: 'rk3588_cpu_usage_percent', // labels: cluster, core
  cpuFreq: 'rk3588_cpu_freq_mhz',
  cpuSeconds: 'rk3588_cpu_seconds_total', // label: mode (counter)
  cpuGovernor: 'rk3588_cpu_governor_info',
  fanPwm: 'rk3588_fan_pwm', // label: fan, 0-255
  memory: 'rk3588_memory_bytes', // label: type
  swap: 'rk3588_swap_bytes', // label: type
  ddrLoad: 'rk3588_ddr_load_percent',
  ddrFreq: 'rk3588_ddr_freq_mhz',
  gpuLoad: 'rk3588_gpu_load_percent',
  gpuFreq: 'rk3588_gpu_freq_mhz',
  npuLoad: 'rk3588_npu_load_percent', // label: core
  npuFreq: 'rk3588_npu_freq_mhz',
  rgaLoad: 'rk3588_rga_load_percent', // label: scheduler
  rgaFreq: 'rk3588_rga_freq_mhz',
  vpuLoad: 'rk3588_vpu_load_percent', // label: unit
  vpuFreq: 'rk3588_vpu_freq_mhz',
  vpuSessions: 'rk3588_vpu_sessions',
  fsSize: 'rk3588_filesystem_size_bytes', // labels: mount, fstype, source
  fsUsed: 'rk3588_filesystem_used_bytes',
  fsAvail: 'rk3588_filesystem_avail_bytes',
  zfsOnline: 'rk3588_zfs_pool_online', // label: pool
  diskInfo: 'rk3588_disk_info', // labels: device, model, vendor, bus, rotational
  diskSize: 'rk3588_disk_size_bytes',
  diskBusy: 'rk3588_disk_busy_percent', // label: device
  diskRead: 'rk3588_disk_read_bytes_per_second',
  diskWrite: 'rk3588_disk_write_bytes_per_second',
  diskTemp: 'rk3588_disk_temp_celsius', // labels: device, source
  smartInfo: 'rk3588_smart_info', // labels: device, model, firmware
  netRx: 'rk3588_network_receive_bytes_per_second', // label: iface
  netTx: 'rk3588_network_transmit_bytes_per_second',
  netInfo: 'rk3588_network_info', // labels: iface, kind (ethernet|wifi|vpn|other)
  netSpeed: 'rk3588_network_speed_mbps', // label: iface; wired links that are up
  procs: 'rk3588_procs', // label: state
} as const

type Labels = Record<string, string>

/** All points of a metric (empty when the metric is absent). */
export function samples(snap: Snapshot | null | undefined, name: string): Sample[] {
  return snap?.metrics[name] ?? []
}

function matches(s: Sample, want?: Labels): boolean {
  if (!want) return true
  for (const k in want) if (s.labels?.[k] !== want[k]) return false
  return true
}

/** First point whose labels include every key/value of `want`. */
export function find(snap: Snapshot | null | undefined, name: string, want?: Labels): Sample | undefined {
  return samples(snap, name).find((s) => matches(s, want))
}

/** Value of that point, or undefined when the metric is missing (show "n/a"). */
export function val(snap: Snapshot | null | undefined, name: string, want?: Labels): number | undefined {
  return find(snap, name, want)?.value
}

/** Value of a label on a point ("" when absent). */
export function label(s: Sample, key: string): string {
  return s.labels?.[key] ?? ''
}

/**
 * Labels that identify one series of each charted metric. The history keeps one
 * ring buffer per series; keying by the identifying label keeps keys short
 * (rk3588_cpu_usage_percent{core=4}) and independent of extra labels.
 */
const ID_LABELS: Record<string, string[]> = {
  [M.cpuUsage]: ['core'],
  [M.cpuFreq]: ['core'],
  [M.npuLoad]: ['core'],
  [M.diskTemp]: ['device'],
}

/** Stable key of a series, e.g. `rk3588_temp_celsius{zone=soc}`. */
export function seriesKey(name: string, labels?: Labels): string {
  if (!labels) return name
  const ids = ID_LABELS[name] ?? Object.keys(labels).sort()
  const body = ids.filter((k) => k in labels).map((k) => `${k}=${labels[k]}`).join(',')
  return body ? `${name}{${body}}` : name
}
