import { describe, expect, it } from 'vitest'
import { fmtLinkSpeed, fmtRatePair } from './format'
import { netTag, networkInterfaces } from './network'
import { live, snap } from '../test/fixtures'

describe('networkInterfaces', () => {
  const nets = networkInterfaces(live())

  it('lists wired interfaces before tunnels, with their rates and kind', () => {
    expect(nets.map((n) => n.iface)).toEqual(['eth0', 'tailscale0'])
    expect(nets[0]).toMatchObject({ kind: 'ethernet', speedMbps: 1000, rx: 520000, tx: 640000 })
    expect(nets[1]).toMatchObject({ kind: 'vpn', speedMbps: undefined, rx: 74000 })
  })

  it('an exporter that does not say what an interface is leaves it "other", never a wrong guess', () => {
    const old = snap({ rk3588_network_receive_bytes_per_second: [[{ iface: 'eth0' }, 1]], rk3588_network_transmit_bytes_per_second: [[{ iface: 'eth0' }, 2]] })
    expect(networkInterfaces(old)[0].kind).toBe('other')
    expect(networkInterfaces(null)).toEqual([])
  })

  it('orders several interfaces of a kind by name, with numbers in order', () => {
    const s = snap({
      rk3588_network_receive_bytes_per_second: [[{ iface: 'eth10' }, 1], [{ iface: 'eth2' }, 1], [{ iface: 'wlan0' }, 1]],
      rk3588_network_info: [[{ iface: 'eth10', kind: 'ethernet' }, 1], [{ iface: 'eth2', kind: 'ethernet' }, 1], [{ iface: 'wlan0', kind: 'wifi' }, 1]],
    })
    expect(networkInterfaces(s).map((n) => n.iface)).toEqual(['eth2', 'eth10', 'wlan0'])
  })
})

describe('network labels', () => {
  it('tags an interface by kind and link speed', () => {
    expect(netTag({ kind: 'ethernet', speedMbps: 1000 })).toBe('1 GbE')
    expect(netTag({ kind: 'ethernet', speedMbps: undefined })).toBe('Ethernet') // link down
    expect(netTag({ kind: 'vpn', speedMbps: undefined })).toBe('VPN')
    expect(netTag({ kind: 'wifi', speedMbps: undefined })).toBe('Wi-Fi')
    expect(netTag({ kind: 'other', speedMbps: undefined })).toBe('')
  })

  it('writes the link speed the usual way', () => {
    expect(fmtLinkSpeed(10000)).toBe('10 GbE')
    expect(fmtLinkSpeed(2500)).toBe('2.5 GbE')
    expect(fmtLinkSpeed(1000)).toBe('1 GbE')
    expect(fmtLinkSpeed(100)).toBe('100 Mb/s')
  })

  it('writes download and upload in the unit of the larger one', () => {
    expect(fmtRatePair(520000, 640000)).toEqual({ a: '520', b: '640', unit: 'KB/s' })
    expect(fmtRatePair(500000, 1_400_000)).toEqual({ a: '0.5', b: '1.4', unit: 'MB/s' })
    expect(fmtRatePair(2e9, 1e8)).toEqual({ a: '2.00', b: '0.10', unit: 'GB/s' })
    expect(fmtRatePair(0, 0)).toEqual({ a: '0', b: '0', unit: 'KB/s' })
  })
})
