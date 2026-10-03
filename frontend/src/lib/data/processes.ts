// What the process table needs on top of the list the server sends.

import type { ProcessRow, ProcessSort } from '../api/types'

/**
 * The server's default order of each column: biggest first for CPU, memory and threads,
 * A→Z / lowest first for the others. The first click on a column gives this order, the
 * second flips it.
 */
const DEFAULT_DESCENDING: Record<ProcessSort, boolean> = {
  cpu: true,
  mem: true,
  threads: true,
  pid: false,
  name: false,
  user: false,
}

/** Whether the list is shown biggest-first (▾) rather than smallest-first (▴). */
export const isDescending = (sort: ProcessSort, reverse: boolean): boolean => DEFAULT_DESCENDING[sort] !== reverse

/** The command column: kernel threads have no command line, so they show their name in brackets like top and btop do. */
export const commandOf = (p: Pick<ProcessRow, 'cmd' | 'name'>): string => p.cmd || `[${p.name}]`
