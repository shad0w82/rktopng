// The network interfaces as the Network section shows them.

import type { Snapshot } from '../api/types'
import { fmtLinkSpeed } from './format'
import { M, label, samples, val } from './metrics'

export type NetKind = 'ethernet' | 'wifi' | 'vpn' | 'other'

export interface NetInterface {
  iface: string
  kind: NetKind
  /** Negotiated speed of a wired link that is up. */
  speedMbps: number | undefined
  /** Bytes per second. */
  rx: number | undefined
  tx: number | undefined
}

const KINDS: readonly NetKind[] = ['ethernet', 'wifi', 'vpn', 'other']
const rank = (k: NetKind) => KINDS.indexOf(k)

/** The interfaces that have a throughput, wired first, then Wi-Fi, then tunnels, each group by name. */
export function networkInterfaces(s: Snapshot | null): NetInterface[] {
  const kindOf = (iface: string): NetKind => {
    const k = label(samples(s, M.netInfo).find((x) => label(x, 'iface') === iface) ?? { value: 0 }, 'kind')
    return (KINDS as readonly string[]).includes(k) ? (k as NetKind) : 'other'
  }
  return samples(s, M.netRx)
    .map((x) => label(x, 'iface'))
    .map((iface) => ({
      iface,
      kind: kindOf(iface),
      speedMbps: val(s, M.netSpeed, { iface }),
      rx: val(s, M.netRx, { iface }),
      tx: val(s, M.netTx, { iface }),
    }))
    .sort((a, b) => rank(a.kind) - rank(b.kind) || a.iface.localeCompare(b.iface, undefined, { numeric: true }))
}

/** The small tag after the interface name: "1 GbE", "Wi-Fi", "VPN". */
export function netTag(i: Pick<NetInterface, 'kind' | 'speedMbps'>): string {
  switch (i.kind) {
    case 'ethernet':
      return i.speedMbps ? fmtLinkSpeed(i.speedMbps) : 'Ethernet'
    case 'wifi':
      return 'Wi-Fi'
    case 'vpn':
      return 'VPN'
    default:
      return ''
  }
}
