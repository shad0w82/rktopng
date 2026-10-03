// The Storage section's view of the disks and volumes: which volumes belong together,
// which disks have a temperature sensor, and the live I/O of each disk.

import type { Snapshot } from '../api/types'
import type { Volume } from './derive'
import type { Disk } from './inventory'
import { M, val } from './metrics'
import { busLabel } from './names'

export interface VolumeGroup {
  /** Vertical tag of the group: ZFS, eMMC, NVMe, SATA… */
  label: string
  volumes: Volume[]
}

/** Buses whose disks have no temperature sensor (eMMC and SD cards expose none). */
const NO_SENSOR = new Set(['emmc', 'sd'])
export const hasTempSensor = (d: Disk): boolean => !NO_SENSOR.has(d.bus)

/** Buses whose disks answer S.M.A.R.T. (the exporter reads exactly these); eMMC and SD cards do not. */
const SMART_BUSES = new Set(['nvme', 'sata', 'scsi', 'usb'])
export const hasSmart = (d: Disk): boolean => SMART_BUSES.has(d.bus)

/**
 * What a volume lives on, as a short tag. A ZFS pool is "ZFS"; a volume on a /dev
 * partition takes the bus of its disk (nvme0n1p1 → NVMe, mmcblk2p9 → eMMC); any
 * other file system is named by its type.
 *
 * The root file system of the FriendlyElec images is an overlay (the real root is
 * not visible from the running system), kept on the eMMC: with an eMMC present, an
 * overlay is therefore tagged eMMC. That is a convention of these boards, not something
 * the kernel reports.
 */
export function volumeGroupLabel(v: Volume, disks: Disk[]): string {
  if (v.fstype === 'zfs') return 'ZFS'
  if (v.source.startsWith('/dev/')) {
    const dev = v.source.slice('/dev/'.length)
    // the longest disk name the device starts with (nvme0n1p1 → nvme0n1, sda1 → sda)
    const disk = disks.filter((d) => dev.startsWith(d.device)).sort((a, b) => b.device.length - a.device.length)[0]
    if (disk) return busLabel(disk.bus)
  }
  if (v.fstype === 'overlay' && disks.some((d) => d.bus === 'emmc')) return busLabel('emmc')
  return v.fstype.toUpperCase()
}

/** Volumes grouped by what they live on: ZFS first, then the others in order of appearance. */
export function volumeGroups(volumes: Volume[], disks: Disk[]): VolumeGroup[] {
  const groups: VolumeGroup[] = []
  for (const v of volumes) {
    const label = volumeGroupLabel(v, disks)
    const g = groups.find((x) => x.label === label)
    if (g) g.volumes.push(v)
    else groups.push({ label, volumes: [v] })
  }
  return groups.sort((a, b) => Number(b.label === 'ZFS') - Number(a.label === 'ZFS'))
}

export interface IoRow {
  device: string
  /** % of the time the disk was busy. */
  busy: number | undefined
  /** Read + write, bytes per second. */
  rate: number | undefined
}

export interface IoGroup {
  bus: string
  label: string
  rows: IoRow[]
}

/** The disks grouped by bus (in the inventory's order) with their live busy % and total throughput. */
export function ioGroups(disks: Disk[], s: Snapshot | null): IoGroup[] {
  const groups: IoGroup[] = []
  for (const d of disks) {
    let g = groups.find((x) => x.bus === d.bus)
    if (!g) groups.push((g = { bus: d.bus, label: busLabel(d.bus), rows: [] }))
    const read = val(s, M.diskRead, { device: d.device })
    const write = val(s, M.diskWrite, { device: d.device })
    g.rows.push({
      device: d.device,
      busy: val(s, M.diskBusy, { device: d.device }),
      rate: read === undefined && write === undefined ? undefined : (read ?? 0) + (write ?? 0),
    })
  }
  return groups
}
