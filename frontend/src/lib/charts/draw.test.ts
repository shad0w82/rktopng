import { describe, expect, it } from 'vitest'
import { clamp, drawFan, drawIO, drawLines, drawNet, drawStack, drawTemp, niceMax } from './draw'
import type { Palette } from './theme'

const colors: Palette = { info: '#5aa2f0', accent: '#34d0bd', good: '#46c25a', warn: '#e6b34a', orange: '#eb8b3a', crit: '#f2564d', faint: '#5c6875' }

/** A context that records what was drawn; enough to check the shape of a chart without a browser. */
function recorder() {
  const calls: string[] = []
  const texts: string[] = []
  const fns = {
    beginPath: 'beginPath',
    moveTo: 'moveTo',
    lineTo: 'lineTo',
    closePath: 'closePath',
    stroke: 'stroke',
    fill: 'fill',
    fillRect: 'fillRect',
  }
  const g: Record<string, unknown> = {
    measureText: () => ({ width: 20 }),
    createLinearGradient: () => ({ addColorStop: () => {} }),
    fillText: (t: string) => texts.push(t),
  }
  for (const [name, tag] of Object.entries(fns)) g[name] = () => calls.push(tag)
  return { g: g as unknown as CanvasRenderingContext2D, calls, texts }
}

describe('clamp', () => {
  it('keeps a value inside the range', () => {
    expect(clamp(-5, 0, 100)).toBe(0)
    expect(clamp(150, 0, 100)).toBe(100)
    expect(clamp(42, 0, 100)).toBe(42)
  })
})

describe('drawFan', () => {
  it('draws the gridlines, the area, the line and the 0/25/50/75 % labels', () => {
    const { g, calls, texts } = recorder()
    drawFan(g, 300, 94, [0, 10, 40, 80, 100, 55], colors)
    expect(calls.filter((c) => c === 'stroke')).toHaveLength(3 + 1) // 25/50/75 gridlines + the line
    expect(calls).toContain('fill') // the soft area under the line
    expect(texts).toEqual(['0%', '25%', '50%', '75%']) // no 100 % label: that line is the tile's top edge
  })

  it('draws nothing until there are at least two samples', () => {
    const { g, calls } = recorder()
    drawFan(g, 300, 94, [], colors)
    drawFan(g, 300, 94, [50], colors)
    expect(calls).toEqual([])
  })

  it('tolerates values outside 0-100 (clamped to the tile)', () => {
    const { g } = recorder()
    expect(() => drawFan(g, 300, 94, [-20, 250, 50], colors)).not.toThrow()
  })
})

describe('drawLines', () => {
  it('draws the four gridlines and one line per series', () => {
    const { g, calls } = recorder()
    drawLines(g, 300, 94, [
      { data: [0, 20, 40], color: '#5aa2f0' },
      { data: [10, 10, 10], color: '#34d0bd' },
    ])
    expect(calls.filter((c) => c === 'stroke')).toHaveLength(4 + 2)
  })

  it('skips a series with fewer than two samples instead of dividing by zero', () => {
    const { g, calls } = recorder()
    drawLines(g, 300, 94, [{ data: [50], color: '#fff' }])
    expect(calls.filter((c) => c === 'stroke')).toHaveLength(4) // gridlines only
  })
})

describe('drawStack', () => {
  it('fills one polygon per band', () => {
    const { g, calls } = recorder()
    drawStack(g, 300, 96, [
      { data: [10, 20, 30], color: 'a' },
      { data: [5, 5, 5], color: 'b' },
      { data: [], color: 'c', rest: true },
    ])
    expect(calls.filter((c) => c === 'fill')).toHaveLength(3)
  })

  it('draws nothing without at least two samples', () => {
    const { g, calls } = recorder()
    drawStack(g, 300, 96, [])
    drawStack(g, 300, 96, [{ data: [10], color: 'a' }])
    expect(calls).toEqual([])
  })
})

describe('drawTemp', () => {
  it('draws the 40/65/90 gridlines, the area, the line and the 15/40/65/90 labels', () => {
    const { g, calls, texts } = recorder()
    drawTemp(g, 300, 76, [44, 45, 47, 46, 52], colors)
    expect(calls.filter((c) => c === 'stroke')).toHaveLength(3 + 1)
    expect(calls).toContain('fill')
    expect(texts).toEqual(['15°', '40°', '65°', '90°']) // no 115° label: that line is the top edge
  })

  it('draws nothing until there are two samples, and clamps readings outside 15-115', () => {
    const empty = recorder()
    drawTemp(empty.g, 300, 76, [44], colors)
    expect(empty.calls).toEqual([])
    const { g } = recorder()
    expect(() => drawTemp(g, 300, 76, [-10, 300, 50], colors)).not.toThrow()
  })
})

describe('niceMax', () => {
  it('rounds up to the next 1-2-5 step', () => {
    expect(niceMax(1e6)).toBe(1e6)
    expect(niceMax(1.2e6)).toBe(2e6)
    expect(niceMax(3e6)).toBe(5e6)
    expect(niceMax(7e6)).toBe(1e7)
    expect(niceMax(4.2e8)).toBe(5e8)
  })
})

describe('drawIO', () => {
  it('draws four gridlines, the read and the write line, and the scale in the top band', () => {
    const { g, calls, texts } = recorder()
    drawIO(g, 300, 94, [0, 2e6, 3e6], [0, 1e6, 0], colors)
    expect(calls.filter((c) => c === 'stroke')).toHaveLength(4 + 2)
    expect(texts).toEqual(['5.0 MB/s']) // the axis tops at the 1-2-5 step above 3 MB/s
  })

  it('never scales below 1 MB/s, so an idle disk shows a flat line instead of noise', () => {
    const { g, texts } = recorder()
    drawIO(g, 300, 94, [0, 10, 20], [0, 0, 5], colors)
    expect(texts).toEqual(['1.0 MB/s'])
  })

  it('draws nothing without two samples', () => {
    const { g, calls } = recorder()
    drawIO(g, 300, 94, [5], [5], colors)
    expect(calls).toEqual([])
  })
})

describe('drawNet', () => {
  it('draws the download and the upload line and nothing else', () => {
    const { g, calls, texts } = recorder()
    drawNet(g, 300, 64, [0, 50, 20], [0, 5, 10], colors)
    expect(calls.filter((c) => c === 'stroke')).toHaveLength(2)
    expect(texts).toEqual([])
  })

  it('draws nothing without two samples and survives an all-zero history', () => {
    const empty = recorder()
    drawNet(empty.g, 300, 64, [1], [1], colors)
    expect(empty.calls).toEqual([])
    const flat = recorder()
    expect(() => drawNet(flat.g, 300, 64, [0, 0, 0], [0, 0, 0], colors)).not.toThrow()
  })
})
