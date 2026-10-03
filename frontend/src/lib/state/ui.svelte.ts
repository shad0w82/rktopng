// Mobile or desktop interface: two separate UIs, picked from the viewport and
// touch input, with a manual override that is remembered on this device.

export type UiMode = 'mobile' | 'desktop'

const STORAGE_KEY = 'rktopng.ui'

/** Storage can be unavailable (private window, blocked site data): never let it throw. */
function readOverride(): UiMode | null {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    return v === 'mobile' || v === 'desktop' ? v : null
  } catch {
    return null
  }
}

function writeOverride(mode: UiMode | null): void {
  try {
    if (mode) localStorage.setItem(STORAGE_KEY, mode)
    else localStorage.removeItem(STORAGE_KEY)
  } catch {
    // not remembered; the choice still holds for this session
  }
}

/** Phone-sized viewport, or a touch device with a narrow-ish screen (tablets in portrait). */
export function detectMode(width: number, coarsePointer: boolean): UiMode {
  return width <= 760 || (coarsePointer && width < 1024) ? 'mobile' : 'desktop'
}

export class UiStore {
  override = $state<UiMode | null>(readOverride())
  private auto = $state<UiMode>('desktop')
  mode = $derived<UiMode>(this.override ?? this.auto)

  constructor() {
    if (typeof window === 'undefined') return
    const update = () => {
      this.auto = detectMode(window.innerWidth, window.matchMedia('(pointer: coarse)').matches)
    }
    update()
    window.addEventListener('resize', update)
  }

  /** Force a UI, or pass null to go back to automatic. */
  set(mode: UiMode | null): void {
    this.override = mode
    writeOverride(mode)
  }
}

export const ui = new UiStore()
