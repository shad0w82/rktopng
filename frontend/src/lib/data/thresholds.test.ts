import { describe, expect, it } from 'vitest'
import { cpuLevel, diskLevel, levelColor, loadLevel, ramLevel, tempLevel } from './thresholds'

// Tables from docs/color_thresholds.md. Lower limits are inclusive.
describe('four-band thresholds', () => {
  it('CPU %: <50 good · 50–74 warn · 75–89 orange · ≥90 crit', () => {
    expect([49.9, 50, 74.9, 75, 89.9, 90].map(cpuLevel)).toEqual(['good', 'warn', 'warn', 'orange', 'orange', 'crit'])
  })
  it('RAM %: <60 good · 60–79 warn · 80–91 orange · ≥92 crit', () => {
    expect([59.9, 60, 79.9, 80, 91.9, 92].map(ramLevel)).toEqual(['good', 'warn', 'warn', 'orange', 'orange', 'crit'])
  })
  it('temperature °C: <55 good · 55–69 warn · 70–84 orange · ≥85 crit', () => {
    expect([54.9, 55, 69.9, 70, 84.9, 85].map(tempLevel)).toEqual(['good', 'warn', 'warn', 'orange', 'orange', 'crit'])
  })
  it('volume capacity %: <70 good · 70–84 warn · 85–94 orange · ≥95 crit', () => {
    expect([69.9, 70, 84.9, 85, 94.9, 95].map(diskLevel)).toEqual(['good', 'warn', 'warn', 'orange', 'orange', 'crit'])
  })
})

describe('loadLevel (three bands, no orange)', () => {
  it('<50 good · 50–79 warn · ≥80 crit', () => {
    expect([0, 49.9, 50, 79.9, 80, 100].map(loadLevel)).toEqual(['good', 'good', 'warn', 'warn', 'crit', 'crit'])
  })
})

it('levelColor points at the CSS variables', () => {
  expect(levelColor('orange')).toBe('var(--orange)')
})
