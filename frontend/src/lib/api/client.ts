import type { ProcessList, ProcessSort, Snapshot, SmartDetail } from './types'

// The UI is served by the exporter itself, so every URL is same-origin (no CORS).
// In development, Vite proxies /api to a real exporter (see vite.config.ts).
//
// Every address here is RELATIVE ("api/info", never "/api/info"): the page may live
// under a sub-path behind a reverse proxy (https://host/rktopng/), and a relative
// address follows it wherever it is, with no setting and no rebuild. Do not start
// one with a slash.

async function getJSON<T>(url: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(url, { signal, headers: { Accept: 'application/json' } })
  if (!res.ok) throw new Error(`${url}: HTTP ${res.status}`)
  return (await res.json()) as T
}

/** Static board and disk identity. Ask once when the page opens. */
export function fetchInfo(signal?: AbortSignal): Promise<Snapshot> {
  return getJSON<Snapshot>('api/info', signal)
}

/** Sortable process list; call it only while the processes section is visible. */
export function fetchProcesses(sort: ProcessSort, limit: number, reverse = false, signal?: AbortSignal): Promise<ProcessList> {
  return getJSON<ProcessList>(`api/processes?sort=${sort}&limit=${limit}${reverse ? '&reverse=1' : ''}`, signal)
}

/** Full S.M.A.R.T. report of one disk (nvme0n1, sda, …), read when asked: call it when the window opens. */
export function fetchSmart(device: string, signal?: AbortSignal): Promise<SmartDetail> {
  return getJSON<SmartDetail>(`api/smart/${encodeURIComponent(device)}`, signal)
}

export interface StreamHandlers {
  onSnapshot(snapshot: Snapshot): void
  /** The connection (re)opened. */
  onOpen?(): void
  /** The connection dropped; EventSource keeps retrying by itself. */
  onError?(): void
}

type EventSourceCtor = new (url: string) => EventSource

/**
 * Follow the live metrics (Server-Sent Events). Returns a function that closes
 * the stream. The browser reconnects automatically after a drop; the first event
 * after a reconnection restores the state.
 */
export function openStream(
  handlers: StreamHandlers,
  url = 'api/stream',
  EventSourceImpl: EventSourceCtor = EventSource,
): () => void {
  const es = new EventSourceImpl(url)
  es.onopen = () => handlers.onOpen?.()
  es.onerror = () => handlers.onError?.()
  es.onmessage = (ev: MessageEvent<string>) => {
    try {
      handlers.onSnapshot(JSON.parse(ev.data) as Snapshot)
    } catch {
      // a malformed frame must not kill the stream: skip it
    }
  }
  return () => es.close()
}
