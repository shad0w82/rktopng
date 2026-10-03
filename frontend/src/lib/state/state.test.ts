import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ProcessList, SmartDetail } from '../api/types'
import { live } from '../test/fixtures'
import { K } from '../data/history'
import { ConnectionStore, LiveStore, ProcessStore, SmartStore, STALE_AFTER_MS } from './stores.svelte'
import { UiStore, detectMode } from './ui.svelte'

describe('ConnectionStore', () => {
  it('goes live on the first event and flags a stalled stream', () => {
    const c = new ConnectionStore()
    expect(c.status).toBe('connecting')
    c.markEvent(1000)
    expect(c.status).toBe('live')

    c.check(1000 + STALE_AFTER_MS) // not yet: the limit is exclusive
    expect(c.status).toBe('live')
    c.check(1000 + STALE_AFTER_MS + 1)
    expect(c.status).toBe('stale')
  })

  it('reports recovery so the caller can reload the identity', () => {
    const c = new ConnectionStore()
    expect(c.markEvent(1)).toBe(false) // first connection: nothing was down
    c.markError()
    expect(c.status).toBe('offline')
    expect(c.markEvent(2)).toBe(true) // back after an outage
    expect(c.status).toBe('live')
    expect(c.markEvent(3)).toBe(false)
  })

  it('does not call a connection that never opened stale', () => {
    const c = new ConnectionStore()
    c.check(10 ** 9)
    expect(c.status).toBe('connecting')
  })
})

describe('LiveStore', () => {
  it('keeps the previous snapshot for counters, bumps the tick and feeds the history', () => {
    const s = new LiveStore()
    const a = live({ timestamp: 1 })
    const b = live({ timestamp: 2, cpuSeconds: { user: 1040, nice: 0, system: 510, idle: 8050, iowait: 100, irq: 0, softirq: 50 } })
    s.push(a, {})
    expect(s.previous).toBeNull()
    s.push(b, {})
    expect(s.snapshot).toBe(b)
    expect(s.previous).toBe(a)
    expect(s.tick).toBe(2)
    expect(s.history.get(K.cpuAvg)).toHaveLength(40)
    expect(s.history.get(K.cpuMode('user')).at(-1)).toBeCloseTo(40, 6) // needed two snapshots
  })
})

describe('ProcessStore', () => {
  afterEach(() => vi.unstubAllGlobals())

  const list = (sort: string, pids: number[]): ProcessList => ({
    timestamp: 1,
    sort,
    total: 400,
    processes: pids.map((pid) => ({ pid, name: `p${pid}`, user: 'root', state: 'S', cmd: '', cpu: 0, mem_pct: 0, mem_bytes: 0, threads: 1 })),
  })
  const respond = (body: ProcessList) => ({ ok: true, json: async () => body })

  it('a second click on the same column asks the server for the other end; another column resets it', async () => {
    const urls: string[] = []
    vi.stubGlobal('fetch', async (url: string) => {
      urls.push(url)
      const q = new URL(url, 'http://x').searchParams
      return respond(list(q.get('sort')!, q.get('reverse') ? [9, 8, 7] : [1, 2, 3]))
    })
    const p = new ProcessStore()
    await p.refresh()
    expect(p.rows.map((r) => r.pid)).toEqual([1, 2, 3])
    expect(p.list?.total).toBe(400)

    p.setSort('cpu') // same column → flip
    await vi.waitFor(() => expect(p.rows.map((r) => r.pid)).toEqual([9, 8, 7]))
    expect(p.reverse).toBe(true)
    expect(urls.at(-1)).toContain('reverse=1')

    p.setSort('mem') // new column → default order
    await vi.waitFor(() => expect(p.rows.map((r) => r.pid)).toEqual([1, 2, 3]))
    expect(p.sort).toBe('mem')
    expect(p.reverse).toBe(false)
    expect(urls.at(-1)).not.toContain('reverse')
  })

  it('ignores a slow answer that arrives after a newer request', async () => {
    const pending: Array<(l: ProcessList) => void> = []
    vi.stubGlobal('fetch', () => new Promise((resolve) => pending.push((l) => resolve(respond(l)))))
    const p = new ProcessStore()
    const first = p.refresh()
    const second = p.refresh() // supersedes the first (the first one is aborted)
    pending[1](list('cpu', [7]))
    pending[0](list('cpu', [99])) // late and obsolete
    await Promise.all([first, second])
    expect(p.rows.map((r) => r.pid)).toEqual([7])
  })

  it('reports a failure without throwing', async () => {
    vi.stubGlobal('fetch', async () => ({ ok: false, status: 500, json: async () => ({}) }))
    const p = new ProcessStore()
    await p.refresh()
    expect(p.error).toContain('HTTP 500')
    expect(p.rows).toEqual([])
  })
})

describe('SmartStore', () => {
  afterEach(() => vi.unstubAllGlobals())

  const report = (device: string, over: Partial<SmartDetail> = {}): SmartDetail => ({
    device,
    available: true,
    read_at: 1,
    identity: { model: `model of ${device}` },
    health: { state: 'ok' },
    vitals: { levels: {} },
    logs: {},
    ...over,
  })
  const answer = (body: SmartDetail) => ({ ok: true, json: async () => body })

  it('is closed until a disk is opened; opening reads that disk and remembers who had the focus', async () => {
    const urls: string[] = []
    vi.stubGlobal('fetch', async (url: string) => {
      urls.push(url)
      return answer(report('sdb'))
    })
    const s = new SmartStore()
    expect([s.device, s.status, s.detail]).toEqual([null, 'idle', null])
    const opener = { focus() {} } as unknown as HTMLElement
    const done = s.open('sdb', opener)
    expect([s.device, s.status, s.detail]).toEqual(['sdb', 'loading', null])
    await done
    expect(urls).toEqual(['api/smart/sdb'])
    expect(s.status).toBe('ready')
    expect(s.detail?.identity.model).toBe('model of sdb')
    expect(s.opener).toBe(opener)
  })

  it('a disk that could not be read is still a successful answer, for the window to explain', async () => {
    vi.stubGlobal('fetch', async () => answer(report('sda', { available: false, reason: 'standby', health: { state: 'unknown' } })))
    const s = new SmartStore()
    await s.open('sda')
    expect(s.status).toBe('ready')
    expect(s.detail).toMatchObject({ available: false, reason: 'standby' })
  })

  it('reports a failed request and lets the same window try again', async () => {
    let fail = true
    vi.stubGlobal('fetch', async () => (fail ? { ok: false, status: 503, json: async () => ({}) } : answer(report('sda'))))
    const s = new SmartStore()
    await s.open('sda')
    expect(s.status).toBe('failed')
    expect(s.error).toContain('HTTP 503')
    fail = false
    await s.load()
    expect([s.status, s.error, s.detail?.device]).toEqual(['ready', '', 'sda'])
  })

  it('ignores a slow answer for a disk that is no longer the open one', async () => {
    const pending: Array<(d: SmartDetail) => void> = []
    vi.stubGlobal('fetch', () => new Promise((resolve) => pending.push((d) => resolve(answer(d)))))
    const s = new SmartStore()
    const first = s.open('sda')
    const second = s.open('sdb') // supersedes the first
    pending[1](report('sdb'))
    pending[0](report('sda')) // late and obsolete
    await Promise.all([first, second])
    expect([s.device, s.detail?.device]).toEqual(['sdb', 'sdb'])
  })

  it('closing clears everything and drops an answer that is still on its way', async () => {
    let release: (d: SmartDetail) => void = () => {}
    vi.stubGlobal('fetch', () => new Promise((resolve) => (release = (d) => resolve(answer(d)))))
    const s = new SmartStore()
    const done = s.open('sda')
    s.close()
    release(report('sda'))
    await done
    expect([s.device, s.detail, s.status, s.error]).toEqual([null, null, 'idle', ''])
  })
})

describe('detectMode', () => {
  it('phone-sized viewports are mobile, wide screens desktop', () => {
    expect(detectMode(390, true)).toBe('mobile')
    expect(detectMode(760, false)).toBe('mobile')
    expect(detectMode(761, false)).toBe('desktop')
    expect(detectMode(1440, false)).toBe('desktop')
  })
  it('a touch tablet below 1024 px is mobile, above it desktop', () => {
    expect(detectMode(820, true)).toBe('mobile')
    expect(detectMode(1180, true)).toBe('desktop')
  })
})


describe('UiStore', () => {
  afterEach(() => vi.unstubAllGlobals())

  /** A browser window of this size (touch or not), enough for the store. */
  const window = (innerWidth: number, touch = false) =>
    vi.stubGlobal('window', { innerWidth, matchMedia: () => ({ matches: touch }), addEventListener: () => {} })

  it('picks the interface from the window: a phone-sized one is mobile, a wide one desktop', () => {
    window(390, true)
    expect(new UiStore().mode).toBe('mobile')
    window(700)
    expect(new UiStore().mode).toBe('mobile')
    window(1360)
    expect(new UiStore().mode).toBe('desktop')
  })

  it('a manual choice always wins', () => {
    window(1360)
    const ui = new UiStore()
    ui.set('mobile')
    expect(ui.mode).toBe('mobile')
    ui.set(null)
    expect(ui.mode).toBe('desktop')
  })
})
