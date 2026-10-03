import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchInfo, fetchProcesses, fetchSmart, openStream } from './client'

/** Minimal stand-in for EventSource that tests drive by hand. */
class FakeEventSource {
  static last: FakeEventSource
  onopen: (() => void) | null = null
  onerror: (() => void) | null = null
  onmessage: ((ev: { data: string }) => void) | null = null
  closed = false
  constructor(readonly url: string) {
    FakeEventSource.last = this
  }
  close() {
    this.closed = true
  }
}
const Impl = FakeEventSource as unknown as new (url: string) => EventSource

describe('openStream', () => {
  it('hands parsed snapshots to the caller and reports open/error', () => {
    const onSnapshot = vi.fn()
    const onOpen = vi.fn()
    const onError = vi.fn()
    openStream({ onSnapshot, onOpen, onError }, 'api/stream', Impl)
    const es = FakeEventSource.last
    expect(es.url).toBe('api/stream')

    es.onopen?.()
    es.onmessage?.({ data: JSON.stringify({ timestamp: 5, metrics: { a: [{ value: 1 }] } }) })
    es.onerror?.()

    expect(onOpen).toHaveBeenCalledOnce()
    expect(onSnapshot).toHaveBeenCalledWith({ timestamp: 5, metrics: { a: [{ value: 1 }] } })
    expect(onError).toHaveBeenCalledOnce()
  })

  it('skips a malformed frame instead of breaking the stream', () => {
    const onSnapshot = vi.fn()
    openStream({ onSnapshot }, 'api/stream', Impl)
    const es = FakeEventSource.last
    es.onmessage?.({ data: '{not json' })
    es.onmessage?.({ data: JSON.stringify({ timestamp: 1, metrics: {} }) })
    expect(onSnapshot).toHaveBeenCalledOnce()
  })

  it('returns a function that closes the connection', () => {
    const close = openStream({ onSnapshot: vi.fn() }, 'api/stream', Impl)
    close()
    expect(FakeEventSource.last.closed).toBe(true)
  })
})

describe('fetch helpers', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('asks the right URLs', async () => {
    const fetchMock = vi.fn(async () => ({ ok: true, json: async () => ({ timestamp: 1, metrics: {} }) }))
    vi.stubGlobal('fetch', fetchMock)
    await fetchInfo()
    await fetchProcesses('mem', 20)
    await fetchProcesses('pid', 15, true)
    expect(fetchMock.mock.calls.map((c) => (c as unknown[])[0])).toEqual([
      'api/info',
      'api/processes?sort=mem&limit=20',
      'api/processes?sort=pid&limit=15&reverse=1',
    ])
  })

  it('turns an HTTP error into an exception', async () => {
    vi.stubGlobal('fetch', async () => ({ ok: false, status: 404, json: async () => ({}) }))
    await expect(fetchInfo()).rejects.toThrow('HTTP 404')
  })
})

describe('fetchSmart', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('asks for one disk, encoded, and returns the report', async () => {
    const urls: string[] = []
    vi.stubGlobal('fetch', async (url: string) => {
      urls.push(url)
      return { ok: true, json: async () => ({ device: 'nvme0n1', available: true }) }
    })
    expect(await fetchSmart('nvme0n1')).toMatchObject({ device: 'nvme0n1', available: true })
    await fetchSmart('a/b c')
    expect(urls).toEqual(['api/smart/nvme0n1', 'api/smart/a%2Fb%20c'])
  })

  it('turns an unknown disk (404) into an error', async () => {
    vi.stubGlobal('fetch', async () => ({ ok: false, status: 404, json: async () => ({ error: 'unknown disk' }) }))
    await expect(fetchSmart('zzz')).rejects.toThrow('HTTP 404')
  })
})
