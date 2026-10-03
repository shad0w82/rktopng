export interface Surface {
  g: CanvasRenderingContext2D
  /** Size in CSS pixels: draw code works in these, the bitmap is scaled for sharp lines on dense screens. */
  w: number
  h: number
}

/** Size the canvas bitmap to its on-screen box (times the device pixel ratio) and clear it. */
export function prepare(cv: HTMLCanvasElement): Surface | null {
  const dpr = window.devicePixelRatio || 1
  const w = cv.clientWidth
  const h = cv.clientHeight
  if (!w || !h) return null // hidden or not laid out yet
  const bw = Math.round(w * dpr)
  const bh = Math.round(h * dpr)
  if (cv.width !== bw) cv.width = bw
  if (cv.height !== bh) cv.height = bh
  const g = cv.getContext('2d')
  if (!g) return null
  g.setTransform(dpr, 0, 0, dpr, 0, 0)
  g.clearRect(0, 0, w, h)
  return { g, w, h }
}
