// Canvas cannot use CSS variables, so the chart colours are read once from the
// design tokens in app.css and handed to the draw functions as plain strings.

export interface Palette {
  info: string
  accent: string
  good: string
  warn: string
  orange: string
  crit: string
  faint: string
}

let cached: Palette | undefined

/** The token colours as `#rrggbb` strings (the draw code appends an alpha byte to them, e.g. `col + '4d'`). */
export function palette(): Palette {
  if (cached) return cached
  const css = getComputedStyle(document.documentElement)
  const c = (name: string) => css.getPropertyValue(name).trim()
  const p: Palette = {
    info: c('--info'),
    accent: c('--accent'),
    good: c('--good'),
    warn: c('--warn'),
    orange: c('--orange'),
    crit: c('--crit'),
    faint: c('--faint'),
  }
  // Before the stylesheet has loaded the tokens are empty: do not remember that.
  if (p.info) cached = p
  return p
}
