// Number formatting shared by every section. Same rules as the approved mockups.

const KB = 1024
const MB = KB * 1024
const GB = MB * 1024
const TB = GB * 1024

/** Compact value: one decimal below 10, none from 10 up ("9.5", "10"). */
const compact = (x: number, unit: string): string => (x < 10 ? x.toFixed(1) : String(Math.round(x))) + unit

/** Capacity in binary units, at most 5 characters: "98M", "9.5G", "10G", "1.8T". */
export function fmtSize(bytes: number): string {
  if (bytes <= 0) return '0K'
  if (bytes >= TB) return compact(bytes / TB, 'T')
  if (bytes >= GB) return compact(bytes / GB, 'G')
  if (bytes >= MB) return compact(bytes / MB, 'M')
  return compact(bytes / KB, 'K')
}

/**
 * Throughput in decimal units: "12 KB/s", "10.3 MB/s", "124 MB/s", "1.2 GB/s".
 * With `idleLabel`, anything under 1 KB/s reads "idle" instead of "0 KB/s".
 */
export function fmtRate(bytesPerSecond: number, idleLabel = false): string {
  const v = bytesPerSecond
  if (v < 1e3) return idleLabel ? 'idle' : '0 KB/s'
  if (v < 1e6) return `${Math.round(v / 1e3)} KB/s`
  if (v < 1e8) return `${(v / 1e6).toFixed(1)} MB/s`
  if (v < 1e9) return `${Math.round(v / 1e6)} MB/s`
  return `${(v / 1e9).toFixed(1)} GB/s`
}

/** Resident memory of a process, at most 5 characters: "999M", "1.2G", "15.3G". */
export function fmtProcMem(bytes: number): string {
  const m = bytes / MB
  return m < 1000 ? `${Math.round(m)}M` : `${(m / 1024).toFixed(1)}G`
}

/** Uptime as "6d 22h 50m", dropping the leading zero units. */
export function fmtUptime(seconds: number): string {
  const s = Math.max(0, Math.floor(seconds))
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (d > 0) return `${d}d ${h}h ${m}m`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

export const fmtMHz = (mhz: number): string => `${Math.round(mhz)} MHz`

/**
 * Two rates written in one shared unit, chosen by the larger: download and upload of an
 * interface sit side by side and share the unit printed at the end of the row.
 * KB/s without decimals, MB/s with one, GB/s with two (decimal units, like fmtRate).
 */
export function fmtRatePair(a: number, b: number): { a: string; b: string; unit: string } {
  const m = Math.max(a, b)
  const [div, unit, digits] = m < 1e6 ? [1e3, 'KB/s', 0] : m < 1e9 ? [1e6, 'MB/s', 1] : [1e9, 'GB/s', 2]
  return { a: (a / div).toFixed(digits), b: (b / div).toFixed(digits), unit }
}

/** Ethernet link speed as it is usually written: "10 GbE", "2.5 GbE", "1 GbE", "100 Mb/s". */
export function fmtLinkSpeed(mbps: number): string {
  if (mbps >= 1000) return `${Number((mbps / 1000).toFixed(1))} GbE`
  return `${mbps} Mb/s`
}

/** Gigabytes with one decimal, always: "4.8G", "11.0G" (the memory legends). */
export const fmtGiB = (bytes: number): string => `${(bytes / GB).toFixed(1)}G`

/**
 * The RAM that is fitted, "16 GB". The kernel reports a bit less (firmware and GPU
 * reservations: 15.6 GiB on a 16 GB board), so the size is rounded up to whole gigabytes.
 */
export const fmtInstalledRam = (totalBytes: number): string => `${Math.ceil(totalBytes / GB - 0.01)} GB`
