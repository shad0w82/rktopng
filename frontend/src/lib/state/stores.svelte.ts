// Reactive state of the dashboard (Svelte 5 runes). The stores only hold data and
// run the I/O; every calculation lives in lib/data as pure functions.

import { fetchInfo, fetchProcesses, fetchSmart } from '../api/client'
import type { ProcessList, ProcessRow, ProcessSort, SmartDetail, Snapshot } from '../api/types'
import { History } from '../data/history'
import { busMap, parseBoard, parseDisks } from '../data/inventory'

/** Board and disk identity, loaded once (/api/info). */
export class InfoStore {
  snapshot = $state.raw<Snapshot | null>(null)
  error = $state('')
  board = $derived(parseBoard(this.snapshot))
  disks = $derived(parseDisks(this.snapshot))
  /** device → bus, to group the live per-disk metrics. */
  busOf = $derived(busMap(this.disks))

  async load(): Promise<void> {
    try {
      this.snapshot = await fetchInfo()
      this.error = ''
    } catch (e) {
      this.error = e instanceof Error ? e.message : String(e)
    }
  }
}

/** The latest live snapshot (one per second) and the short history behind the charts. */
export class LiveStore {
  snapshot = $state.raw<Snapshot | null>(null)
  /** The one before `snapshot`: counters (CPU time) need two readings to become a rate. */
  previous = $state.raw<Snapshot | null>(null)
  /** Bumped on every push; canvas charts redraw from an effect that reads it. */
  tick = $state(0)
  readonly history = new History()

  push(next: Snapshot, busOf: Record<string, string>): void {
    this.history.record(next, this.snapshot, busOf)
    this.previous = this.snapshot
    this.snapshot = next
    this.tick++
  }
}

export type ConnStatus = 'connecting' | 'live' | 'stale' | 'offline'

/** Silence longer than this (ms) while "live" means the stream stalled. */
export const STALE_AFTER_MS = 4000

export class ConnectionStore {
  status = $state<ConnStatus>('connecting')
  lastEventAt = $state(0)

  /** An event arrived. Returns true when the connection had been down (state may have changed). */
  markEvent(now = Date.now()): boolean {
    const wasDown = this.status === 'offline' || this.status === 'stale'
    this.status = 'live'
    this.lastEventAt = now
    return wasDown
  }

  markError(): void {
    this.status = 'offline'
  }

  /** Called every second: flags a stream that went quiet without an error. */
  check(now = Date.now()): void {
    if (this.status === 'live' && now - this.lastEventAt > STALE_AFTER_MS) this.status = 'stale'
  }
}

/** The sortable process table (/api/processes), polled only while it is on screen. */
export class ProcessStore {
  list = $state.raw<ProcessList | null>(null)
  sort = $state<ProcessSort>('cpu')
  /** Second click on the same column flips the server's default order (it returns the other end of the whole list). */
  reverse = $state(false)
  error = $state('')
  /** Rows asked of the server: the desktop table shows 15, the phone's cards 10. */
  limit = 15
  rows = $derived<ProcessRow[]>(this.list?.processes ?? [])

  private timer: ReturnType<typeof setInterval> | undefined
  private inflight: AbortController | undefined

  setSort(key: ProcessSort): void {
    if (key === this.sort) this.reverse = !this.reverse
    else {
      this.sort = key
      this.reverse = false
    }
    void this.refresh()
  }

  async refresh(): Promise<void> {
    this.inflight?.abort() // a newer request supersedes an older one
    const ctl = new AbortController()
    this.inflight = ctl
    try {
      const list = await fetchProcesses(this.sort, this.limit, this.reverse, ctl.signal)
      if (this.inflight === ctl) {
        this.list = list
        this.error = ''
      }
    } catch (e) {
      if (ctl.signal.aborted) return
      this.error = e instanceof Error ? e.message : String(e)
    }
  }

  /** Start polling (call when the processes section becomes visible). */
  start(intervalMs = 2000): void {
    if (this.timer) return
    void this.refresh()
    this.timer = setInterval(() => void this.refresh(), intervalMs)
  }

  stop(): void {
    if (this.timer) clearInterval(this.timer)
    this.timer = undefined
    this.inflight?.abort()
  }
}

export type SmartStatus = 'idle' | 'loading' | 'ready' | 'failed'

/**
 * The S.M.A.R.T. window of one disk (/api/smart/<device>). The disk is read once, when the window opens
 * (the exporter keeps the report for a few seconds), not polled. `detail.available` can still be false:
 * the request worked but the disk could not be read (standby, no permission…).
 */
export class SmartStore {
  /** The disk whose window is open; null = closed. */
  device = $state<string | null>(null)
  detail = $state.raw<SmartDetail | null>(null)
  status = $state<SmartStatus>('idle')
  error = $state('')
  /** What had the focus before the window opened, so closing puts it back. */
  opener: HTMLElement | null = null

  private inflight: AbortController | undefined

  open(device: string, opener: HTMLElement | null = null): Promise<void> {
    this.opener = opener
    this.device = device
    this.detail = null
    return this.load()
  }

  /** (Re)read the open disk; also the "try again" of a failed read. */
  async load(): Promise<void> {
    const device = this.device
    if (!device) return
    this.inflight?.abort() // a newer request supersedes an older one
    const ctl = new AbortController()
    this.inflight = ctl
    this.status = 'loading'
    this.error = ''
    try {
      const detail = await fetchSmart(device, ctl.signal)
      if (this.inflight === ctl && this.device === device) {
        this.detail = detail
        this.status = 'ready'
      }
    } catch (e) {
      if (ctl.signal.aborted) return
      this.error = e instanceof Error ? e.message : String(e)
      this.status = 'failed'
    }
  }

  close(): void {
    this.inflight?.abort()
    this.device = null
    this.detail = null
    this.status = 'idle'
    this.error = ''
  }
}
