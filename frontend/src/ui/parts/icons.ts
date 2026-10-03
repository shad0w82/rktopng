// Section icons (the same ones as the mobile tabs), as the inner markup of a 24×24 stroke SVG.
// Constants written here, never built from data, so they are safe to inject with {@html}.

export const ICONS = {
  overview:
    '<rect x="3.5" y="3.5" width="7" height="7" rx="1.3"/><rect x="13.5" y="3.5" width="7" height="7" rx="1.3"/><rect x="3.5" y="13.5" width="7" height="7" rx="1.3"/><rect x="13.5" y="13.5" width="7" height="7" rx="1.3"/>',
  cpu: '<rect x="7" y="7" width="10" height="10" rx="1.5"/><path d="M10 3v3M14 3v3M10 18v3M14 18v3M3 10h3M3 14h3M18 10h3M18 14h3"/>',
  temperature: '<path d="M10 13.6V5a2 2 0 1 1 4 0v8.6a4 4 0 1 1-4 0z"/>',
  accelerators: '<path d="M13 2 4 13h6l-1 9 9-11h-6z"/>',
  memory:
    '<rect x="2.5" y="8" width="19" height="9" rx="1.5"/><path d="M6.5 8V5.8M10.5 8V5.8M14.5 8V5.8M18.5 8V5.8M6 17v2.2M18 17v2.2"/>',
  storage:
    '<ellipse cx="12" cy="5.5" rx="7.5" ry="2.8"/><path d="M4.5 5.5v13c0 1.55 3.36 2.8 7.5 2.8s7.5-1.25 7.5-2.8v-13"/><path d="M4.5 12c0 1.55 3.36 2.8 7.5 2.8s7.5-1.25 7.5-2.8"/>',
  network: '<path d="M7 16.5 3.5 13 7 9.5M3.5 13H14M17 7.5 20.5 11 17 14.5M20.5 11H10"/>',
  processes: '<path d="M8.5 6h12M8.5 12h12M8.5 18h12M3.5 6h.01M3.5 12h.01M3.5 18h.01"/>',
} as const

export type IconName = keyof typeof ICONS
