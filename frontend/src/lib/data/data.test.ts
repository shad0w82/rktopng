import { describe, expect, it } from 'vitest'
import {
  cmaUsage,
  cpuAverage,
  cpuGovernor,
  diskTemperatures,
  fanLevel,
  loadBreakdown,
  memBreakdown,
  ramUsedPct,
  swapUsage,
  throughputByBus,
  volumes,
  zfsSummary,
  zoneTemperatures,
} from './derive'
import { History, K } from './history'
import { busMap, parseBoard, parseDisks } from './inventory'
import { M, find, seriesKey, val } from './metrics'
import { RingBuffer } from './ringbuffer'
import { CPU_GROUPS, LINE_COLORS, busLabel, busRank, rgaLabel, vpuLabel, zoneLabel } from './names'
import { info, live, snap } from '../test/fixtures'

const GB = 1024 ** 3

describe('metrics helpers', () => {
  const s = live()
  it('finds a point by a subset of its labels', () => {
    expect(val(s, M.cpuUsage, { core: '3' })).toBe(16)
    expect(find(s, M.cpuUsage, { cluster: 'big', core: '3' })).toBeUndefined()
    expect(val(s, M.temp, { zone: 'npu' })).toBe(47)
  })
  it('returns undefined (→ "n/a") for a missing metric or snapshot', () => {
    expect(val(s, 'rk3588_nope')).toBeUndefined()
    expect(val(null, M.cpuUsage)).toBeUndefined()
  })
  it('builds stable series keys from the identifying labels', () => {
    expect(seriesKey(M.cpuUsage, { cluster: 'big', core: '4' })).toBe('rk3588_cpu_usage_percent{core=4}')
    expect(seriesKey(M.temp, { zone: 'soc' })).toBe('rk3588_temp_celsius{zone=soc}')
    expect(seriesKey(M.diskTemp, { device: 'sda', source: 'smart' })).toBe('rk3588_disk_temp_celsius{device=sda}')
    expect(seriesKey(M.ddrLoad)).toBe('rk3588_ddr_load_percent')
  })
})

describe('RingBuffer', () => {
  it('starts flat, keeps the newest values in order and drops the oldest', () => {
    const b = new RingBuffer(3, 7)
    expect(b.toArray()).toEqual([7, 7, 7])
    b.push(1)
    b.push(2)
    expect(b.toArray()).toEqual([7, 1, 2])
    b.push(3)
    b.push(4)
    expect(b.toArray()).toEqual([2, 3, 4])
    expect(b.last()).toBe(4)
  })
})

describe('derived values', () => {
  const s = live()
  it('CPU average is the mean of the cores', () => expect(cpuAverage(s)).toBe(18))
  it('RAM used is (total − available) / total, not counting the cache', () => {
    expect(ramUsedPct(s)).toBeCloseTo(((15.6 - 10.8) / 15.6) * 100, 6)
  })
  it('memory breakdown always sums to the total', () => {
    const m = memBreakdown(s)!
    expect(m.used + m.cache + m.free).toBeCloseTo(m.total, 0)
    expect(m.cache).toBeCloseTo(3.5 * GB, -3) // cached + buffers
    expect(m.free).toBeGreaterThan(0)
  })
  it('cache never exceeds what is actually available', () => {
    const odd = snap({
      rk3588_memory_bytes: [
        [{ type: 'total' }, 10 * GB],
        [{ type: 'available' }, 1 * GB],
        [{ type: 'cached' }, 6 * GB],
        [{ type: 'buffers' }, 0],
      ],
    })
    const m = memBreakdown(odd)!
    expect(m.cache).toBe(1 * GB)
    expect(m.free).toBe(0)
  })
  it('fan: PWM 0-255 → %', () => {
    const f = fanLevel(s)!
    expect(f.raw).toBe(64)
    expect(f.pct).toBeCloseTo(25.1, 1)
  })
  it('swap and CMA usage; absent data gives undefined', () => {
    expect(swapUsage(s)).toEqual({ total: 0, used: 0, pct: 0 })
    expect(cmaUsage(s)!.pct).toBeCloseTo((14 / 128) * 100, 6)
    expect(cmaUsage(snap({}))).toBeUndefined()
    expect(ramUsedPct(null)).toBeUndefined()
  })
})

describe('volumes', () => {
  const v = volumes(live())
  it('lists "/" first, then by mount point', () => {
    expect(v.map((x) => x.mount)).toEqual(['/', '/DATA', '/SSD_Pool'])
  })
  it('computes the percentage like df: used / (used + avail)', () => {
    expect(v[0].pct).toBeCloseTo(20.4, 1) // df says 21% on the real board; size-based would give 19.3%
    expect(v[0].pct).not.toBeCloseTo((v[0].used / v[0].size) * 100, 1)
  })
  it('keeps the pool name as source', () => expect(v[1].source).toBe('NVME_Pool'))
  it('summarises ZFS pools', () => {
    expect(zfsSummary(live())).toEqual({ online: 2, total: 2 })
    expect(zfsSummary(snap({}))).toBeUndefined()
    const degraded = snap({ rk3588_zfs_pool_online: [[{ pool: 'a' }, 1], [{ pool: 'b' }, 0]] })
    expect(zfsSummary(degraded)).toEqual({ online: 1, total: 2 })
  })
})

describe('loadBreakdown (counter → %)', () => {
  const t0 = live({ cpuSeconds: { user: 1000, nice: 0, system: 500, idle: 8000, iowait: 100, irq: 0, softirq: 50 } })
  it('splits the time elapsed between two snapshots', () => {
    const t1 = live({ cpuSeconds: { user: 1030, nice: 10, system: 515, idle: 8040, iowait: 105, irq: 0, softirq: 0 } })
    // softirq went backwards (50 → 0): the exporter restarted, so there is no valid rate
    expect(loadBreakdown(t0, t1)).toBeUndefined()
    const ok = live({ cpuSeconds: { user: 1030, nice: 10, system: 515, idle: 8040, iowait: 105, irq: 0, softirq: 55 } })
    const b = loadBreakdown(t0, ok)!
    // deltas: user 40 · system 15+5=20 · iowait 5 · idle 40 → total 105
    expect(b.user).toBeCloseTo((40 / 105) * 100, 6)
    expect(b.system).toBeCloseTo((20 / 105) * 100, 6)
    expect(b.iowait).toBeCloseTo((5 / 105) * 100, 6)
    expect(b.idle).toBeCloseTo((40 / 105) * 100, 6)
    expect(b.user + b.system + b.iowait + b.idle).toBeCloseTo(100, 6)
  })
  it('needs two snapshots and some elapsed time', () => {
    expect(loadBreakdown(null, t0)).toBeUndefined()
    expect(loadBreakdown(t0, t0)).toBeUndefined()
  })
})

describe('inventory', () => {
  it('reads the board identity', () => {
    expect(parseBoard(info)).toMatchObject({ soc: 'RK3588', os: 'Ubuntu 24.04.4 LTS', npuDriver: 'v0.9.8' })
    expect(parseBoard(null)).toBeUndefined()
  })
  const disks = parseDisks(info)
  it('orders disks by bus (NVMe, SATA, eMMC) then by name with numbers in order', () => {
    expect(disks.map((d) => d.device)).toEqual(['nvme0n1', 'nvme1n1', 'nvme10n1', 'sda', 'sdb', 'mmcblk2'])
  })
  it('joins size and SMART identity; missing parts stay empty', () => {
    const sda = disks.find((d) => d.device === 'sda')!
    expect(sda).toMatchObject({ firmware: 'SVQ02B6Q', rotational: false })
    expect(sda.sizeBytes).toBeCloseTo(1.8 * 1024 * GB, -6)
    const sdb = disks.find((d) => d.device === 'sdb')!
    expect(sdb.firmware).toBe('') // no SMART identity yet
    expect(sdb.model).toBe('Samsung SSD 860') // the kernel's name until SMART answers
    expect(sda.model).toBe('Samsung SSD 870 QVO 2TB') // the full name from SMART
    expect(sdb.sizeBytes).toBeUndefined()
  })
  it('maps devices to buses', () => expect(busMap(disks)).toMatchObject({ nvme0n1: 'nvme', sda: 'sata', mmcblk2: 'emmc' }))
})

describe('diskTemperatures', () => {
  const t = diskTemperatures(parseDisks(info), live())
  it('keeps the inventory order (NVMe, SATA, eMMC) and joins bus and size', () => {
    expect(t.map((x) => x.device)).toEqual(['nvme0n1', 'nvme1n1', 'nvme10n1', 'sda', 'sdb', 'mmcblk2'])
    expect(t[0]).toMatchObject({ bus: 'nvme', celsius: 37.85 })
    expect(t[0].sizeBytes).toBeGreaterThan(0)
  })
  it('a disk without a reading (eMMC, or SATA before the first SMART poll) is n/a, not 0', () => {
    expect(t.find((x) => x.device === 'mmcblk2')?.celsius).toBeUndefined()
    expect(t.find((x) => x.device === 'sdb')?.celsius).toBeUndefined()
  })
  it('works before the identity or the first push have arrived', () => {
    expect(diskTemperatures([], live())).toEqual([])
    expect(diskTemperatures(parseDisks(info), null).every((x) => x.celsius === undefined)).toBe(true)
  })
})

describe('CPU layout and governor', () => {
  it('the three groups list each of the 8 RK3588 cores exactly once, in order', () => {
    expect(CPU_GROUPS.flatMap((g) => g.cores)).toEqual([0, 1, 2, 3, 4, 5, 6, 7])
    expect(CPU_GROUPS.map((g) => g.label)).toEqual(['A55', 'A76', 'A76'])
  })
  it('has a line colour for every core of the biggest group', () => {
    expect(LINE_COLORS.length).toBeGreaterThanOrEqual(Math.max(...CPU_GROUPS.map((g) => g.cores.length)))
  })
  it('reads the governor, or nothing when it is not exported', () => {
    const withGov = snap({ rk3588_cpu_governor_info: [[{ governor: 'ondemand' }, 1]] })
    expect(cpuGovernor(withGov)).toBe('ondemand')
    expect(cpuGovernor(live())).toBeUndefined()
    expect(cpuGovernor(null)).toBeUndefined()
  })
})

describe('zoneTemperatures', () => {
  it('lists the seven zones in display order with their UI names', () => {
    expect(zoneTemperatures(live()).map((z) => z.label)).toEqual(['soc', 'little', 'big_0', 'big_1', 'center', 'gpu', 'npu'])
  })
  it('a zone the kernel does not report stays n/a (undefined), not 0', () => {
    const z = zoneTemperatures(live())
    expect(z.find((x) => x.zone === 'soc')?.celsius).toBe(46.2)
    expect(z.find((x) => x.zone === 'npu')?.celsius).toBe(47)
    expect(z.find((x) => x.zone === 'center')?.celsius).toBeUndefined()
    expect(zoneTemperatures(null).every((x) => x.celsius === undefined)).toBe(true)
  })
})

describe('throughputByBus', () => {
  it('sums the read and write of the disks on each bus', () => {
    const t = throughputByBus(live(), busMap(parseDisks(info)))
    expect(t.nvme).toEqual({ read: 7e6, write: 4e6 })
    expect(t.sata).toEqual({ read: 2e6, write: 1e6 })
    expect(t.emmc).toEqual({ read: 0, write: 1e5 })
  })
  it('ignores disks it knows nothing about', () => {
    expect(throughputByBus(live(), { sda: 'sata' })).toEqual({ sata: { read: 1e6, write: 5e5 } })
  })
})

describe('names', () => {
  it('translates driver names to UI names and passes unknown ones through', () => {
    expect(vpuLabel('enc_core0')).toBe('enc0')
    expect(vpuLabel('av1_decoder')).toBe('av1d')
    expect(vpuLabel('something_new')).toBe('something_new')
    expect(rgaLabel('rga2_2')).toBe('rga2')
    expect(zoneLabel('bigcore0')).toBe('big_0')
    expect(busLabel('emmc')).toBe('eMMC')
    expect(busLabel('thunderbolt')).toBe('thunderbolt')
    expect(busRank('nvme')).toBeLessThan(busRank('sata'))
    expect(busRank('sata')).toBeLessThan(busRank('emmc'))
  })
})

describe('History', () => {
  it('records raw series, derived ones and per-bus throughput', () => {
    const h = new History(5)
    const busOf = busMap(parseDisks(info))
    const a = live({ cpu: [10, 10, 10, 10, 10, 10, 10, 10] })
    const b = live({
      cpu: [20, 20, 20, 20, 20, 20, 20, 20],
      cpuSeconds: { user: 1040, nice: 0, system: 510, idle: 8050, iowait: 100, irq: 0, softirq: 50 },
    })
    h.record(a, null, busOf)
    h.record(b, a, busOf)

    expect(h.get(seriesKey(M.cpuUsage, { core: '4' }))).toEqual([0, 0, 0, 10, 20]) // loads start from zero
    expect(h.get(K.cpuAvg).at(-1)).toBe(20)
    expect(h.get(seriesKey(M.temp, { zone: 'soc' })).at(-1)).toBe(46.2)
    expect(h.get(K.ramUsed).at(-1)).toBeCloseTo(((15.6 - 10.8) / 15.6) * 100, 6)
    expect(h.get(K.bus('nvme', 'read')).at(-1)).toBe(7e6)
    expect(h.get(K.fanPct).at(-1)).toBeCloseTo((64 / 255) * 100, 6) // PWM 0-255 → %
    // the load breakdown needs a previous snapshot: only the second record adds a point
    expect(h.get(K.cpuMode('user'))).toHaveLength(5)
    expect(h.get(K.cpuMode('user')).at(-1)).toBeCloseTo((40 / 100) * 100, 6)
  })
  it('keeps only `capacity` points and knows nothing of unseen series', () => {
    const h = new History(3)
    for (const t of [30, 31, 32, 33]) h.record(snap({ rk3588_temp_celsius: [[{ zone: 'soc' }, t]] }), null, {})
    expect(h.get(seriesKey(M.temp, { zone: 'soc' }))).toEqual([31, 32, 33])
    expect(h.get('never.seen')).toEqual([])
  })
  it('temperatures start flat at their first value; loads start from zero', () => {
    const h = new History(4)
    h.record(
      snap({
        rk3588_temp_celsius: [[{ zone: 'soc' }, 46]],
        rk3588_cpu_usage_percent: [[{ cluster: 'big', core: '4' }, 83]], // a startup spike must not become a plateau
        rk3588_gpu_load_percent: [[null, 12]],
      }),
      null,
      {},
    )
    expect(h.get(seriesKey(M.temp, { zone: 'soc' }))).toEqual([46, 46, 46, 46])
    expect(h.get(seriesKey(M.cpuUsage, { core: '4' }))).toEqual([0, 0, 0, 83])
    expect(h.get(M.gpuLoad)).toEqual([0, 0, 0, 12])
  })
  it('RAM share starts flat; the CPU breakdown starts as all idle', () => {
    const h = new History(3)
    const a = live()
    const b = live({ cpuSeconds: { user: 1040, nice: 0, system: 510, idle: 8050, iowait: 100, irq: 0, softirq: 50 } })
    h.record(a, null, {})
    h.record(b, a, {})
    const ram = h.get(K.ramUsed)
    expect(ram[0]).toBe(ram[2])
    expect(h.get(K.cpuMode('idle'))).toEqual([100, 100, 50])
    expect(h.get(K.cpuMode('user'))).toEqual([0, 0, 40])
  })
})
