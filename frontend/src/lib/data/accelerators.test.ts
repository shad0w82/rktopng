import { describe, expect, it } from 'vitest'
import { acceleratorDrivers, accelerators, shortDriver } from './accelerators'
import { History } from './history'
import { parseBoard } from './inventory'
import { info, live } from '../test/fixtures'

const group = (s: ReturnType<typeof accelerators>, key: string) => s.groups.find((g) => g.key === key)!

describe('accelerators', () => {
  const a = accelerators(live())

  it('lists NPU, VPU, RGA and GPU in the order of the mockup', () => {
    expect(a.groups.map((g) => g.key)).toEqual(['npu', 'vpu', 'rga', 'gpu'])
    expect(a.groups.map((g) => g.engines.length)).toEqual([3, 4, 3, 1])
  })

  it('every engine of a group has a line colour', () => {
    for (const g of a.groups) expect(g.colors.length).toBeGreaterThanOrEqual(g.engines.length)
  })

  it('NPU: per-core load, one shared clock', () => {
    const npu = group(a, 'npu').engines
    expect(npu.map((e) => e.load)).toEqual([12, 0, 4])
    expect(npu.every((e) => e.freq === 1000)).toBe(true)
  })

  it('VPU: encoders have a clock, decoders do not (shown as a dash)', () => {
    const vpu = group(a, 'vpu').engines
    expect(vpu.map((e) => e.label)).toEqual(['enc0', 'enc1', 'dec0', 'dec1'])
    expect(vpu[0]).toMatchObject({ load: 35, freq: 786.43 })
    expect(vpu[2]).toMatchObject({ load: 80, freq: undefined })
    expect(vpu[3].load).toBeUndefined() // dec1 not exported by this snapshot
  })

  it('VPU extra engines and open sessions', () => {
    expect(a.vpuExtra.map((e) => e.label)).toEqual(['jpeg0', 'jpeg1', 'jpeg2', 'jpeg3', 'jpegd', 'av1d', 'vdpu', 'avsd', 'iep'])
    expect(a.vpuExtra[0].load).toBe(5)
    expect(a.vpuSessions).toBe(2)
  })

  it('RGA: three schedulers with the UI names, each with its own clock', () => {
    const rga = group(a, 'rga').engines
    expect(rga.map((e) => e.label)).toEqual(['rga3·0', 'rga3·1', 'rga2'])
    expect(rga.map((e) => e.load)).toEqual([20, undefined, 0])
    expect(rga.every((e) => e.freq === 750)).toBe(true)
  })

  it('GPU: a single engine', () => {
    expect(group(a, 'gpu').engines[0]).toMatchObject({ label: 'Mali-G610', load: 33, freq: 700 })
  })

  it('without data every load is undefined (n/a), never 0', () => {
    const none = accelerators(null)
    expect(none.groups.flatMap((g) => g.engines).every((e) => e.load === undefined && e.freq === undefined)).toBe(true)
    expect(none.vpuSessions).toBeUndefined()
  })

  it('each engine\'s series key is the one the history records its load under', () => {
    const h = new History(3)
    h.record(live(), null, {})
    for (const g of a.groups) {
      for (const e of g.engines) {
        if (e.load !== undefined) expect(h.get(e.series).at(-1), `${g.key} ${e.label}`).toBe(e.load)
      }
    }
    for (const e of a.vpuExtra) if (e.load !== undefined) expect(h.get(e.series).at(-1)).toBe(e.load)
  })
})

describe('driver versions', () => {
  it('shortens what each driver reports', () => {
    expect(shortDriver('v0.9.8')).toBe('v0.9.8')
    expect(shortDriver('v1.3.10')).toBe('v1.3.10')
    expect(shortDriver('g29p0-00eac0 (UK version 1.36)')).toBe('g29p0')
    expect(shortDriver('c79104d97229 author: Yandong Lin 2025-08-25 video: rockchip: mpp: Fix it')).toBe('c79104d')
    expect(shortDriver('')).toBe('')
  })

  it('gives each accelerator its short form and a tooltip', () => {
    const d = acceleratorDrivers(parseBoard(info))
    expect(Object.fromEntries(Object.entries(d).map(([k, v]) => [k, v.short]))).toEqual({
      npu: 'v0.9.8',
      vpu: 'c79104d',
      rga: 'v1.3.10',
      gpu: 'g29p0',
    })
    expect(d.vpu?.full).toBe('driver MPP c79104d97229 · 2025-08-25') // hash and date, not the commit message
    expect(d.gpu?.full).toBe('driver Mali g29p0-00eac0 (UK version 1.36)')
    expect(d.npu?.full).toBe('driver NPU v0.9.8')
  })

  it('leaves out a driver that is not known (RGA without root) instead of showing an empty note', () => {
    const board = { ...parseBoard(info)!, rgaDriver: '', vpuDriver: '' }
    expect(Object.keys(acceleratorDrivers(board)).sort()).toEqual(['gpu', 'npu'])
    expect(acceleratorDrivers(undefined)).toEqual({})
  })
})
