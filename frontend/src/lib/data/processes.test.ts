import { describe, expect, it } from 'vitest'
import { commandOf, isDescending } from './processes'

describe('process table helpers', () => {
  it('shows the arrow of the order actually on screen', () => {
    expect(isDescending('cpu', false)).toBe(true) // biggest first
    expect(isDescending('cpu', true)).toBe(false)
    expect(isDescending('pid', false)).toBe(false) // lowest first
    expect(isDescending('name', true)).toBe(true) // Z→A
    expect(isDescending('threads', false)).toBe(true)
  })

  it('a kernel thread without a command line shows its name in brackets', () => {
    expect(commandOf({ cmd: '/usr/bin/dockerd -H fd://', name: 'dockerd' })).toBe('/usr/bin/dockerd -H fd://')
    expect(commandOf({ cmd: '', name: 'kworker/0:1' })).toBe('[kworker/0:1]')
  })
})
