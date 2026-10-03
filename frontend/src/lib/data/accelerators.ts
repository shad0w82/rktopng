// The RK3588 accelerators as the Accelerators section shows them: NPU, VPU, RGA
// and GPU, each a list of engines with its load and frequency.

import type { Snapshot } from '../api/types'
import type { Board } from './inventory'
import { M, seriesKey, val } from './metrics'
import { GPU_LABEL, NPU_CORES, RGA, VPU_EXTRA, VPU_MAIN, type LineColor } from './names'

export interface Engine {
  label: string
  /** Busy %, undefined when not exported (the NPU and RGA load need root). */
  load: number | undefined
  /** MHz; undefined for engines without a readable clock (most video engines). */
  freq: number | undefined
  /** Key of its load history (see History). */
  series: string
}

export type AcceleratorKey = 'npu' | 'vpu' | 'rga' | 'gpu'

export interface AcceleratorGroup {
  key: AcceleratorKey
  label: string
  engines: Engine[]
  /** Line colour of each engine in the group's Load chart, in order. */
  colors: LineColor[]
}

export interface Accelerators {
  groups: AcceleratorGroup[]
  /** The rarely used video engines (JPEG, AV1, VDPU…), shown behind the "+" expander. */
  vpuExtra: Engine[]
  /** Open codec sessions (a client such as ffmpeg using the video engines). */
  vpuSessions: number | undefined
}

export function accelerators(s: Snapshot | null): Accelerators {
  const npuFreq = val(s, M.npuFreq) // one clock for all the cores
  const vpu = (unit: string, label: string): Engine => ({
    label,
    load: val(s, M.vpuLoad, { unit }),
    freq: val(s, M.vpuFreq, { unit }),
    series: seriesKey(M.vpuLoad, { unit }),
  })

  return {
    groups: [
      {
        key: 'npu',
        label: 'NPU',
        colors: ['info', 'accent', 'warn'],
        engines: NPU_CORES.map(({ core, label }) => ({
          label,
          load: val(s, M.npuLoad, { core }),
          freq: npuFreq,
          series: seriesKey(M.npuLoad, { core }),
        })),
      },
      { key: 'vpu', label: 'VPU', colors: ['info', 'accent', 'orange', 'good'], engines: VPU_MAIN.map((u) => vpu(u.unit, u.label)) },
      {
        key: 'rga',
        label: 'RGA',
        colors: ['info', 'accent', 'warn'],
        engines: RGA.map(({ scheduler, label }) => ({
          label,
          load: val(s, M.rgaLoad, { scheduler }),
          freq: val(s, M.rgaFreq, { scheduler }),
          series: seriesKey(M.rgaLoad, { scheduler }),
        })),
      },
      {
        key: 'gpu',
        label: 'GPU',
        colors: ['info'],
        engines: [{ label: GPU_LABEL, load: val(s, M.gpuLoad), freq: val(s, M.gpuFreq), series: seriesKey(M.gpuLoad) }],
      },
    ],
    vpuExtra: VPU_EXTRA.map((u) => vpu(u.unit, u.label)),
    vpuSessions: val(s, M.vpuSessions),
  }
}

export interface DriverVersion {
  /** What the tile shows: "v0.9.8", "c79104d", "g29p0". */
  short: string
  /** The tooltip: the whole version, with what it is. */
  full: string
}

/**
 * Short form of a driver version string. A source commit (what the video-codec
 * driver reports) is cut to 7 characters like `git log --abbrev`; the Mali version
 * "g29p0-00eac0 (UK version 1.36)" keeps only its release, "g29p0".
 */
export function shortDriver(raw: string): string {
  const first = raw.trim().split(/\s+/)[0] ?? ''
  if (/^[0-9a-f]{12,40}$/i.test(first)) return first.slice(0, 7)
  return first.split('-')[0]
}

function fullDriver(name: string, raw: string): string {
  const first = raw.trim().split(/\s+/)[0] ?? ''
  const date = /\d{4}-\d{2}-\d{2}/.exec(raw)?.[0]
  // a commit line carries the author and the commit message: keep the hash and the date
  if (/^[0-9a-f]{12,40}$/i.test(first)) return `driver ${name} ${first}${date ? ` · ${date}` : ''}`
  return `driver ${name} ${raw.trim()}`
}

/** Driver version of each accelerator; a group whose driver is unknown (or unreadable without root) is left out. */
export function acceleratorDrivers(board: Board | undefined): Partial<Record<AcceleratorKey, DriverVersion>> {
  if (!board) return {}
  const out: Partial<Record<AcceleratorKey, DriverVersion>> = {}
  const add = (key: AcceleratorKey, name: string, raw: string) => {
    if (raw.trim()) out[key] = { short: shortDriver(raw), full: fullDriver(name, raw) }
  }
  add('npu', 'NPU', board.npuDriver)
  add('vpu', 'MPP', board.vpuDriver)
  add('rga', 'RGA', board.rgaDriver)
  add('gpu', 'Mali', board.gpuDriver)
  return out
}
