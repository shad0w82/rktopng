import { describe, expect, it } from 'vitest'
import type { Volume } from './derive'
import { volumes } from './derive'
import { parseDisks } from './inventory'
import { hasSmart, hasTempSensor, ioGroups, volumeGroupLabel, volumeGroups } from './storage'
import { info, live } from '../test/fixtures'

const disks = parseDisks(info)
const vol = (over: Partial<Volume>): Volume => ({ mount: '/x', fstype: 'ext4', source: '/dev/sda1', size: 1, used: 0, avail: 1, pct: 0, ...over })

describe('volume groups', () => {
  it('tags ZFS pools, then the overlay root as eMMC (CM3588 convention), ZFS first', () => {
    const groups = volumeGroups(volumes(live()), disks)
    expect(groups.map((g) => g.label)).toEqual(['ZFS', 'eMMC'])
    expect(groups[0].volumes.map((v) => v.mount)).toEqual(['/DATA', '/SSD_Pool'])
    expect(groups[1].volumes.map((v) => v.mount)).toEqual(['/'])
  })

  it('a volume on a partition takes the bus of its disk', () => {
    expect(volumeGroupLabel(vol({ source: '/dev/nvme0n1p1' }), disks)).toBe('NVMe')
    expect(volumeGroupLabel(vol({ source: '/dev/nvme10n1p2' }), disks)).toBe('NVMe') // nvme10n1, not nvme1n1
    expect(volumeGroupLabel(vol({ source: '/dev/sda1' }), disks)).toBe('SATA')
    expect(volumeGroupLabel(vol({ source: '/dev/mmcblk2p9' }), disks)).toBe('eMMC')
  })

  it('anything else is named by its file system type; an overlay without an eMMC stays an overlay', () => {
    expect(volumeGroupLabel(vol({ source: '/dev/sdz1' }), disks)).toBe('EXT4')
    expect(volumeGroupLabel(vol({ fstype: 'btrfs', source: '/dev/unknown' }), [])).toBe('BTRFS')
    expect(volumeGroupLabel(vol({ fstype: 'overlay', source: 'overlay' }), [])).toBe('OVERLAY')
  })

  it('no volumes, no groups', () => expect(volumeGroups([], disks)).toEqual([]))
})

describe('temperature sensors', () => {
  it('every disk has one except eMMC and SD', () => {
    expect(disks.filter(hasTempSensor).map((d) => d.device)).toEqual(['nvme0n1', 'nvme1n1', 'nvme10n1', 'sda', 'sdb'])
    // the same disks answer S.M.A.R.T.; the eMMC does not
    expect(disks.filter(hasSmart).map((d) => d.device)).toEqual(['nvme0n1', 'nvme1n1', 'nvme10n1', 'sda', 'sdb'])
  })
})

describe('disk I/O groups', () => {
  const groups = ioGroups(disks, live())

  it('groups the disks by bus in the inventory order', () => {
    expect(groups.map((g) => g.label)).toEqual(['NVMe', 'SATA', 'eMMC'])
    expect(groups[0].rows.map((r) => r.device)).toEqual(['nvme0n1', 'nvme1n1', 'nvme10n1'])
  })

  it('each row carries the read + write of its disk', () => {
    const nvme0 = groups[0].rows[0]
    expect(nvme0.rate).toBe(4e6 + 2e6)
    expect(groups[2].rows[0].rate).toBe(1e5) // eMMC: 0 read + 1e5 write
  })

  it('a disk with no reading is undefined, not zero', () => {
    const nvme10 = groups[0].rows[2]
    expect(nvme10.rate).toBeUndefined()
    expect(nvme10.busy).toBeUndefined()
    expect(ioGroups(disks, null).every((g) => g.rows.every((r) => r.rate === undefined))).toBe(true)
  })
})
