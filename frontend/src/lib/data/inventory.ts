// Static identity of the board and its disks, read once from /api/info.

import type { Snapshot } from '../api/types'
import { M, label, samples, val } from './metrics'
import { busRank } from './names'

export interface Board {
  vendor: string
  soc: string
  model: string
  kernel: string
  os: string
  /** Empty when the exporter is not root (debugfs unreadable). */
  npuDriver: string
  rgaDriver: string
  /** Kernel video-codec (MPP) driver: a source commit, not a release number. */
  vpuDriver: string
  /** Mali kernel driver, e.g. "g29p0-00eac0 (UK version 1.36)". */
  gpuDriver: string
}

export function parseBoard(info: Snapshot | null): Board | undefined {
  const s = samples(info, M.socInfo)[0]
  if (!s) return undefined
  return {
    vendor: label(s, 'vendor'),
    soc: label(s, 'soc'),
    model: label(s, 'model'),
    kernel: label(s, 'kernel'),
    os: label(s, 'os'),
    npuDriver: label(s, 'npu_driver'),
    rgaDriver: label(s, 'rga_driver'),
    vpuDriver: label(s, 'vpu_driver'),
    gpuDriver: label(s, 'gpu_driver'),
  }
}

export interface Disk {
  device: string
  /** Full name from SMART when available ("Samsung SSD 870 QVO 2TB"), else the kernel's, which is cut short ("Samsung SSD 870"). */
  model: string
  vendor: string
  /** nvme | sata | emmc | usb | … (see names.ts for the printed form). */
  bus: string
  rotational: boolean
  sizeBytes: number | undefined
  /** From SMART: empty when the exporter has no SMART access (yet). */
  firmware: string
}

/** Disks grouped by bus, then by name (nvme0n1 before nvme1n1). */
export function parseDisks(info: Snapshot | null): Disk[] {
  return samples(info, M.diskInfo)
    .map((s) => {
      const device = label(s, 'device')
      const smart = samples(info, M.smartInfo).find((x) => label(x, 'device') === device)
      return {
        device,
        model: (smart ? label(smart, 'model') : '') || label(s, 'model'),
        vendor: label(s, 'vendor'),
        bus: label(s, 'bus'),
        rotational: label(s, 'rotational') === '1',
        sizeBytes: val(info, M.diskSize, { device }),
        firmware: smart ? label(smart, 'firmware') : '',
      }
    })
    .sort((a, b) => busRank(a.bus) - busRank(b.bus) || a.device.localeCompare(b.device, undefined, { numeric: true }))
}

/** device → bus, to group live per-disk metrics. */
export function busMap(disks: Disk[]): Record<string, string> {
  return Object.fromEntries(disks.map((d) => [d.device, d.bus]))
}
