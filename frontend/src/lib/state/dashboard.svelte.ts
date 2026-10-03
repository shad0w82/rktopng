// The one place where the stores are wired to the exporter: load the identity
// once, follow the live stream, and keep an eye on the connection.

import { openStream } from '../api/client'
import { ConnectionStore, InfoStore, LiveStore, ProcessStore, SmartStore } from './stores.svelte'

export class Dashboard {
  readonly info = new InfoStore()
  readonly live = new LiveStore()
  readonly conn = new ConnectionStore()
  readonly procs = new ProcessStore()
  readonly smart = new SmartStore()

  private closeStream: (() => void) | undefined
  private watchdog: ReturnType<typeof setInterval> | undefined

  start(): void {
    if (this.closeStream) return
    void this.info.load()
    this.closeStream = openStream({
      onSnapshot: (s) => {
        this.live.push(s, this.info.busOf)
        // After an outage the board may have restarted: re-read its identity.
        if (this.conn.markEvent()) void this.info.load()
      },
      onError: () => this.conn.markError(),
    })
    this.watchdog = setInterval(() => this.conn.check(), 1000)
  }

  stop(): void {
    this.closeStream?.()
    this.closeStream = undefined
    if (this.watchdog) clearInterval(this.watchdog)
    this.watchdog = undefined
    this.procs.stop()
    this.smart.close()
  }
}

export const dashboard = new Dashboard()
