// Which interface a component is drawn in. The desktop and the mobile interface share
// the section components (the cards are the same); the few places where the two differ
// in more than size ask for the layout instead of guessing it from the window width,
// because the interface can be forced on a screen of the other kind.

import { getContext, setContext } from 'svelte'

export type Layout = 'desktop' | 'mobile'

const KEY = Symbol('layout')

/** Called once by the root of an interface. */
export function provideLayout(layout: Layout): void {
  setContext(KEY, layout)
}

/** The layout of the interface this component is in (desktop when nobody said). */
export function useLayout(): Layout {
  return getContext<Layout | undefined>(KEY) ?? 'desktop'
}
