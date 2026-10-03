// Chart drawing, ported from the approved mockups (same scales, gridlines, fades
// and labels). Every function draws into a context of `w` × `h` CSS pixels from a
// series of samples, oldest first; none of them reads application state.

import { fmtRate } from '../data/format'
import type { Palette } from './theme'

export const clamp = (v: number, lo: number, hi: number): number => Math.max(lo, Math.min(hi, v))

const GRID = 'rgba(138,151,165,.16)'
const LABEL_FONT = '600 9px "IBM Plex Mono",monospace'

/** Fade everything drawn so far out toward the left edge, so old samples dissolve behind the tile's text. */
function fadeLeft(g: CanvasRenderingContext2D, w: number, h: number, until: number): void {
  g.globalCompositeOperation = 'destination-out'
  const f = g.createLinearGradient(0, 0, w, 0)
  f.addColorStop(0, 'rgba(0,0,0,1)')
  f.addColorStop(until, 'rgba(0,0,0,0)')
  g.fillStyle = f
  g.fillRect(0, 0, w, h)
  g.globalCompositeOperation = 'source-over'
}

/** A scale label right-aligned at `x`, on a faint backdrop so it stays legible over the curve. */
function scaleLabel(g: CanvasRenderingContext2D, text: string, x: number, baseline: number): void {
  const tw = g.measureText(text).width
  g.fillStyle = 'rgba(10,14,19,.5)'
  g.fillRect(x - tw - 3, baseline - 9, tw + 5, 11)
  g.fillStyle = 'rgba(206,215,223,.95)'
  g.fillText(text, x, baseline)
}

const FAN_TICKS = [0, 25, 50, 75, 100]

/** Fan duty: fixed 0–100 % scale, gridlines at 25/50/75 (0 and 100 are the tile's edges), one blue line over a soft area. */
export function drawFan(g: CanvasRenderingContext2D, w: number, h: number, d: number[], c: Palette): void {
  if (d.length < 2) return
  const col = c.info
  const x = (i: number) => (i / (d.length - 1)) * w
  const y = (v: number) => h - (clamp(v, 0, 100) / 100) * h

  g.strokeStyle = GRID
  g.lineWidth = 1
  for (const t of FAN_TICKS) {
    if (t === 0 || t === 100) continue
    const gy = y(t)
    g.beginPath()
    g.moveTo(0, gy + 0.5)
    g.lineTo(w, gy + 0.5)
    g.stroke()
  }

  const trace = () => d.forEach((v, i) => (i ? g.lineTo(x(i), y(v)) : g.moveTo(x(i), y(v))))
  g.beginPath()
  trace()
  g.lineTo(w, h)
  g.lineTo(0, h)
  g.closePath()
  const grad = g.createLinearGradient(0, 0, 0, h)
  grad.addColorStop(0, col + '4d')
  grad.addColorStop(1, col + '00')
  g.fillStyle = grad
  g.fill()
  g.beginPath()
  trace()
  g.strokeStyle = col
  g.lineWidth = 1.5
  g.stroke()

  fadeLeft(g, w, h, 0.42)

  g.font = LABEL_FONT
  g.textBaseline = 'alphabetic'
  g.textAlign = 'right'
  for (const t of FAN_TICKS) {
    if (t === 100) continue
    scaleLabel(g, t + '%', w - 4, Math.min(h - 2, Math.max(8, y(t) - 4)))
  }
}

export interface Line {
  data: number[]
  color: string
}

/**
 * Utilisation lines on a fixed 0–100 % scale. The top fifth of the tile is kept
 * free for its name and legend (the 100 % gridline is that band's lower edge);
 * the curves fade out toward the left. No scale labels: the scale never changes.
 */
export function drawLines(g: CanvasRenderingContext2D, w: number, h: number, lines: Line[]): void {
  const top = h * 0.2
  const x = (i: number, n: number) => (i / (n - 1)) * w
  const y = (v: number) => h - (clamp(v, 0, 100) / 100) * (h - top)

  g.strokeStyle = GRID
  g.lineWidth = 1
  for (const t of [100, 75, 50, 25]) {
    const gy = y(t)
    g.beginPath()
    g.moveTo(0, gy + 0.5)
    g.lineTo(w, gy + 0.5)
    g.stroke()
  }
  for (const { data, color } of lines) {
    const n = data.length
    if (n < 2) continue
    g.beginPath()
    data.forEach((v, i) => (i ? g.lineTo(x(i, n), y(v)) : g.moveTo(x(i, n), y(v))))
    g.strokeStyle = color
    g.lineWidth = 1.4
    g.stroke()
  }
  fadeLeft(g, w, h, 0.3)
}

export interface Band {
  data: number[]
  color: string
  /** The last band: fills whatever is left up to 100 %, so its own data is ignored. */
  rest?: boolean
}

/** Stacked area on a 0–100 % scale: each band sits on the ones before it. */
export function drawStack(g: CanvasRenderingContext2D, w: number, h: number, bands: Band[]): void {
  const n = bands[0]?.data.length ?? 0
  if (n < 2) return
  const x = (i: number) => (i / (n - 1)) * w
  const y = (v: number) => h - (v / 100) * h
  const below = new Array<number>(n).fill(0)
  for (const band of bands) {
    g.beginPath()
    for (let i = 0; i < n; i++) {
      const top = band.rest ? 100 : below[i] + (band.data[i] ?? 0)
      if (i) g.lineTo(x(i), y(top))
      else g.moveTo(x(i), y(top))
    }
    for (let i = n - 1; i >= 0; i--) g.lineTo(x(i), y(below[i]))
    g.closePath()
    g.fillStyle = band.color
    g.fill()
    if (!band.rest) for (let i = 0; i < n; i++) below[i] += band.data[i] ?? 0
  }
}

const TEMP_TICKS = [15, 40, 65, 90, 115]
const TEMP_LO = 15
const TEMP_HI = 115

/**
 * Temperature history: fixed 15–115 °C scale (gridlines at 40/65/90, 15 and 115 are the
 * tile's edges), a blue line over a soft area, faded toward the left behind the tile's
 * text, with the scale labels on top.
 */
export function drawTemp(g: CanvasRenderingContext2D, w: number, h: number, d: number[], c: Palette): void {
  if (d.length < 2) return
  const col = c.info
  const x = (i: number) => (i / (d.length - 1)) * w
  const y = (v: number) => h - ((clamp(v, TEMP_LO, TEMP_HI) - TEMP_LO) / (TEMP_HI - TEMP_LO)) * h

  g.strokeStyle = 'rgba(138,151,165,.18)'
  g.lineWidth = 1
  for (const t of TEMP_TICKS) {
    if (t === TEMP_LO || t === TEMP_HI) continue
    const gy = y(t)
    g.beginPath()
    g.moveTo(0, gy + 0.5)
    g.lineTo(w, gy + 0.5)
    g.stroke()
  }

  const trace = () => d.forEach((v, i) => (i ? g.lineTo(x(i), y(v)) : g.moveTo(x(i), y(v))))
  g.beginPath()
  trace()
  g.lineTo(w, h)
  g.lineTo(0, h)
  g.closePath()
  const grad = g.createLinearGradient(0, 0, 0, h)
  grad.addColorStop(0, col + '4d')
  grad.addColorStop(1, col + '00')
  g.fillStyle = grad
  g.fill()
  g.beginPath()
  trace()
  g.strokeStyle = col
  g.lineWidth = 1.5
  g.stroke()

  fadeLeft(g, w, h, 0.42)

  g.font = LABEL_FONT
  g.textBaseline = 'alphabetic'
  g.textAlign = 'right'
  for (const t of TEMP_TICKS) {
    if (t === TEMP_HI) continue // that line is the top edge
    scaleLabel(g, t + '°', w - 4, Math.min(h - 2, Math.max(8, y(t) - 4)))
  }
}

/** The next 1-2-5 step at or above v (1, 2, 5, 10, 20, 50…), so an auto-scaled axis does not jitter every second. */
export function niceMax(v: number): number {
  const p = Math.pow(10, Math.floor(Math.log10(v)))
  const m = v / p
  return (m <= 1 ? 1 : m <= 2 ? 2 : m <= 5 ? 5 : 10) * p
}

/**
 * Throughput history: read (blue) and write (teal) lines on an axis that scales to the
 * busiest sample (never below 1 MB/s) in 1-2-5 steps, with the top of the axis written
 * in the free band at the top. Same fade and band as the load charts.
 */
export function drawIO(g: CanvasRenderingContext2D, w: number, h: number, read: number[], write: number[], c: Palette): void {
  const n = read.length
  if (n < 2) return
  const top = h * 0.2
  const max = niceMax(Math.max(1e6, ...read, ...write))
  const x = (i: number) => (i / (n - 1)) * w
  const y = (v: number) => h - (v / max) * (h - top)

  g.strokeStyle = GRID
  g.lineWidth = 1
  for (const t of [1, 0.75, 0.5, 0.25]) {
    const gy = y(max * t)
    g.beginPath()
    g.moveTo(0, gy + 0.5)
    g.lineTo(w, gy + 0.5)
    g.stroke()
  }
  for (const [data, color] of [
    [read, c.info],
    [write, c.accent],
  ] as const) {
    g.beginPath()
    data.forEach((v, i) => (i ? g.lineTo(x(i), y(v)) : g.moveTo(x(i), y(v))))
    g.strokeStyle = color
    g.lineWidth = 1.4
    g.stroke()
  }
  fadeLeft(g, w, h, 0.3)

  g.font = LABEL_FONT
  g.textAlign = 'right'
  g.textBaseline = 'alphabetic'
  scaleLabel(g, fmtRate(max), w - 4, top + 11)
}

/**
 * Network history of one interface: download (blue) and upload (teal) on an axis that
 * scales to the busiest sample (at least 1 B/s), no gridlines, no labels: the numbers are
 * written under the chart.
 */
export function drawNet(g: CanvasRenderingContext2D, w: number, h: number, down: number[], up: number[], c: Palette): void {
  const n = down.length
  if (n < 2) return
  const max = Math.max(1, ...down, ...up)
  const x = (i: number) => (i / (n - 1)) * w
  const y = (v: number) => h - (v / max) * (h - 2) - 1
  for (const [data, color] of [
    [down, c.info],
    [up, c.accent],
  ] as const) {
    g.beginPath()
    data.forEach((v, i) => (i ? g.lineTo(x(i), y(v)) : g.moveTo(x(i), y(v))))
    g.strokeStyle = color
    g.lineWidth = 1.5
    g.stroke()
  }
}
