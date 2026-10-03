// Colour levels of gauges, bars and tiles. Single source of truth for the UI;
// the table is documented in docs/color_thresholds.md. Lower limits are inclusive
// (CPU 50 % is already "warn").

export type Level = 'good' | 'warn' | 'orange' | 'crit'

/** Four bands: below `warn` good, then warn / orange / crit from their limits up. */
const band =
  (warn: number, orange: number, crit: number) =>
  (v: number): Level =>
    v >= crit ? 'crit' : v >= orange ? 'orange' : v >= warn ? 'warn' : 'good'

export const cpuLevel = band(50, 75, 90) // CPU %
export const ramLevel = band(60, 80, 92) // RAM %
export const tempLevel = band(55, 70, 85) // °C: SoC, thermal zones and disks
export const diskLevel = band(70, 85, 95) // volume capacity %

/** Three bands (no orange) for load bars: accelerators, disk busy %, DDR, CMA. */
export function loadLevel(percent: number): Exclude<Level, 'orange'> {
  return percent >= 80 ? 'crit' : percent >= 50 ? 'warn' : 'good'
}

/** CSS colour of a level (the variables live in styles/tokens.css). */
export const levelColor = (level: Level): string => `var(--${level})`
