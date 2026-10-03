import { describe, expect, it } from 'vitest'

// The dashboard can be served under a sub-path (https://host/rktopng/) by a reverse
// proxy. That only works while every address it asks for is relative: an address that
// starts with a slash would leave the sub-path and hit the proxy's root instead.
const sources = import.meta.glob<string>('../../**/*.{ts,svelte}', { query: '?raw', import: 'default', eager: true })
const files = Object.entries(sources).filter(([path]) => !/\.test\.ts$/.test(path))

describe('addresses stay relative', () => {
  it('finds the sources', () => {
    expect(files.length).toBeGreaterThan(20)
  })

  it('has no address that starts with a slash', () => {
    const offenders: string[] = []
    for (const [path, text] of files) {
      text.split('\n').forEach((line: string, i: number) => {
        if (/^\s*(\/\/|\/\*|\*)/.test(line)) return // comments may quote a bad example
        // fetch('/x'), new EventSource('/x'), src="/x", href="/x", or a '/api…', '/assets…', '/favicon…' string
        if (/(?:fetch|EventSource)\s*\(\s*["'`]\/[A-Za-z]/.test(line) || /(?:src|href)=["']\/[A-Za-z]/.test(line) || /["'`]\/(?:api|assets|favicon)/.test(line)) {
          offenders.push(`${path}:${i + 1}: ${line.trim()}`)
        }
      })
    }
    expect(offenders).toEqual([])
  })
})
