// Snapshots shaped like the real exporter's, with values taken from the CM3588
// test board. Only used by tests.

import type { Sample, Snapshot } from '../api/types'

type Labels = Record<string, string>
type Row = [Labels | null, number]

/** Build a snapshot from `name → [[labels, value], …]`. */
export function snap(metrics: Record<string, Row[]>, timestamp = 1790896000000): Snapshot {
  const out: Record<string, Sample[]> = {}
  for (const [name, rows] of Object.entries(metrics)) {
    out[name] = rows.map(([labels, value]) => (labels ? { labels, value } : { value }))
  }
  return { timestamp, metrics: out }
}

const GB = 1024 ** 3

/** What /api/info returns on the test board (root: includes SMART identity). */
export const info = snap({
  rk3588_soc_info: [
    [
      {
        vendor: 'Rockchip',
        soc: 'RK3588',
        model: 'FriendlyElec CM3588',
        kernel: '6.1.141',
        os: 'Ubuntu 24.04.4 LTS',
        npu_driver: 'v0.9.8',
        rga_driver: 'v1.3.10',
        vpu_driver: 'c79104d97229 author: Yandong Lin 2025-08-25 video: rockchip: mpp: Fix load info not clear issue',
        gpu_driver: 'g29p0-00eac0 (UK version 1.36)',
      },
      1,
    ],
  ],
  rk3588_disk_info: [
    [{ device: 'sdb', model: 'Samsung SSD 860', vendor: 'ATA', bus: 'sata', rotational: '0' }, 1],
    [{ device: 'mmcblk2', model: 'A3A561', vendor: '', bus: 'emmc', rotational: '0' }, 1],
    [{ device: 'nvme1n1', model: 'Lexar SSD NM790 1TB', vendor: '', bus: 'nvme', rotational: '0' }, 1],
    [{ device: 'sda', model: 'Samsung SSD 870', vendor: 'ATA', bus: 'sata', rotational: '0' }, 1],
    [{ device: 'nvme0n1', model: 'Lexar SSD NM790 1TB', vendor: '', bus: 'nvme', rotational: '0' }, 1],
    [{ device: 'nvme10n1', model: 'Lexar SSD NM790 1TB', vendor: '', bus: 'nvme', rotational: '0' }, 1],
  ],
  rk3588_disk_size_bytes: [
    [{ device: 'nvme0n1' }, 953.9 * GB],
    [{ device: 'sda' }, 1.8 * 1024 * GB],
    [{ device: 'mmcblk2' }, 57.6 * GB],
  ],
  rk3588_smart_info: [[{ device: 'sda', model: 'Samsung SSD 870 QVO 2TB', firmware: 'SVQ02B6Q' }, 1]],
})

const cores = (vals: number[]): Row[] =>
  vals.map((v, i) => [{ cluster: i < 4 ? 'little' : 'big', core: String(i) }, v] as Row)

/** A live snapshot. `cpuSeconds` lets tests move the counters between two snapshots. */
export function live(opts: { cpu?: number[]; cpuSeconds?: Record<string, number>; timestamp?: number } = {}): Snapshot {
  const modes = opts.cpuSeconds ?? { user: 1000, nice: 0, system: 500, idle: 8000, iowait: 100, irq: 0, softirq: 50 }
  return snap(
    {
      rk3588_uptime_seconds: [[null, 6 * 86400 + 22 * 3600 + 50 * 60]],
      rk3588_cpu_usage_percent: cores(opts.cpu ?? [4, 8, 12, 16, 20, 24, 28, 32]),
      rk3588_cpu_seconds_total: Object.entries(modes).map(([mode, v]) => [{ mode }, v] as Row),
      rk3588_temp_celsius: [
        [{ zone: 'soc' }, 46.2],
        [{ zone: 'npu' }, 47],
      ],
      rk3588_fan_pwm: [[{ fan: '1' }, 64]],
      rk3588_network_receive_bytes_per_second: [
        [{ iface: 'tailscale0' }, 74000],
        [{ iface: 'eth0' }, 520000],
      ],
      rk3588_network_transmit_bytes_per_second: [
        [{ iface: 'tailscale0' }, 107000],
        [{ iface: 'eth0' }, 640000],
      ],
      rk3588_network_info: [
        [{ iface: 'tailscale0', kind: 'vpn' }, 1],
        [{ iface: 'eth0', kind: 'ethernet' }, 1],
      ],
      rk3588_network_speed_mbps: [[{ iface: 'eth0' }, 1000]],
      rk3588_npu_load_percent: [
        [{ core: '0' }, 12],
        [{ core: '1' }, 0],
        [{ core: '2' }, 4],
      ],
      rk3588_npu_freq_mhz: [[null, 1000]],
      rk3588_vpu_load_percent: [
        [{ unit: 'enc_core0' }, 35],
        [{ unit: 'enc_core1' }, 0],
        [{ unit: 'dec_core0' }, 80],
        [{ unit: 'jpeg_enc0' }, 5],
      ],
      rk3588_vpu_freq_mhz: [
        [{ unit: 'enc_core0' }, 786.43],
        [{ unit: 'enc_core1' }, 786.43],
      ],
      rk3588_vpu_sessions: [[null, 2]],
      rk3588_rga_load_percent: [
        [{ scheduler: 'rga3_0' }, 20],
        [{ scheduler: 'rga2_2' }, 0],
      ],
      rk3588_rga_freq_mhz: [
        [{ scheduler: 'rga3_0' }, 750],
        [{ scheduler: 'rga3_1' }, 750],
        [{ scheduler: 'rga2_2' }, 750],
      ],
      rk3588_gpu_load_percent: [[null, 33]],
      rk3588_gpu_freq_mhz: [[null, 700]],
      rk3588_disk_temp_celsius: [
        [{ device: 'nvme0n1', source: 'hwmon' }, 37.85],
        [{ device: 'nvme1n1', source: 'hwmon' }, 34.85],
        [{ device: 'sda', source: 'smart' }, 33],
      ],
      rk3588_memory_bytes: [
        [{ type: 'total' }, 15.6 * GB],
        [{ type: 'available' }, 10.8 * GB],
        [{ type: 'cached' }, 3.4 * GB],
        [{ type: 'buffers' }, 0.1 * GB],
        [{ type: 'cma_total' }, 128 * 1024 * 1024],
        [{ type: 'cma_used' }, 14 * 1024 * 1024],
      ],
      rk3588_swap_bytes: [
        [{ type: 'total' }, 0],
        [{ type: 'used' }, 0],
      ],
      rk3588_filesystem_size_bytes: [
        [{ mount: '/SSD_Pool', fstype: 'zfs', source: 'SSD_Pool' }, 1.931e12],
        [{ mount: '/', fstype: 'overlay', source: 'overlay' }, 55308472320],
        [{ mount: '/DATA', fstype: 'zfs', source: 'NVME_Pool' }, 1.971e12],
      ],
      rk3588_filesystem_used_bytes: [
        [{ mount: '/SSD_Pool', fstype: 'zfs', source: 'SSD_Pool' }, 2.3e8],
        [{ mount: '/', fstype: 'overlay', source: 'overlay' }, 10687959040],
        [{ mount: '/DATA', fstype: 'zfs', source: 'NVME_Pool' }, 1.02e8],
      ],
      rk3588_filesystem_avail_bytes: [
        [{ mount: '/SSD_Pool', fstype: 'zfs', source: 'SSD_Pool' }, 1.93e12],
        [{ mount: '/', fstype: 'overlay', source: 'overlay' }, 41802993664],
        [{ mount: '/DATA', fstype: 'zfs', source: 'NVME_Pool' }, 1.971e12],
      ],
      rk3588_zfs_pool_online: [
        [{ pool: 'NVME_Pool' }, 1],
        [{ pool: 'SSD_Pool' }, 1],
      ],
      rk3588_disk_read_bytes_per_second: [
        [{ device: 'nvme0n1' }, 4e6],
        [{ device: 'nvme1n1' }, 3e6],
        [{ device: 'sda' }, 1e6],
        [{ device: 'sdb' }, 1e6],
        [{ device: 'mmcblk2' }, 0],
      ],
      rk3588_disk_write_bytes_per_second: [
        [{ device: 'nvme0n1' }, 2e6],
        [{ device: 'nvme1n1' }, 2e6],
        [{ device: 'sda' }, 5e5],
        [{ device: 'sdb' }, 5e5],
        [{ device: 'mmcblk2' }, 1e5],
      ],
    },
    opts.timestamp,
  )
}
