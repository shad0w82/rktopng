import { describe, expect, it } from 'vitest'
import { fmtGiB, fmtInstalledRam, fmtMHz, fmtProcMem, fmtRate, fmtSize, fmtUptime } from './format'

const MB = 1024 ** 2
const GB = 1024 ** 3

describe('fmtSize', () => {
  it('keeps one decimal below 10 and none from 10 up', () => {
    expect(fmtSize(98 * MB)).toBe('98M')
    expect(fmtSize(9.5 * GB)).toBe('9.5G')
    expect(fmtSize(10.8 * GB)).toBe('11G')
    expect(fmtSize(1.8 * 1024 * GB)).toBe('1.8T')
  })
  it('matches the capacities shown on the approved mockups', () => {
    expect(fmtSize(953.9 * GB)).toBe('954G')
    expect(fmtSize(57.6 * GB)).toBe('58G')
    expect(fmtSize(52 * GB)).toBe('52G')
  })
  it('handles zero', () => expect(fmtSize(0)).toBe('0K'))
})

describe('fmtRate', () => {
  it('uses decimal units and picks the unit', () => {
    expect(fmtRate(23_400)).toBe('23 KB/s')
    expect(fmtRate(10_300_000)).toBe('10.3 MB/s')
    expect(fmtRate(124_000_000)).toBe('124 MB/s')
    expect(fmtRate(1.2e9)).toBe('1.2 GB/s')
  })
  it('says idle under 1 KB/s only when asked', () => {
    expect(fmtRate(200, true)).toBe('idle')
    expect(fmtRate(200)).toBe('0 KB/s')
  })
})

describe('fmtProcMem', () => {
  it('stays within 5 characters', () => {
    expect(fmtProcMem(119 * MB)).toBe('119M')
    expect(fmtProcMem(1850 * MB)).toBe('1.8G')
    expect(fmtProcMem(15.3 * GB)).toBe('15.3G')
  })
})

describe('fmtUptime', () => {
  it('drops leading zero units', () => {
    expect(fmtUptime(6 * 86400 + 22 * 3600 + 50 * 60)).toBe('6d 22h 50m')
    expect(fmtUptime(5 * 3600 + 12 * 60)).toBe('5h 12m')
    expect(fmtUptime(12 * 60 + 30)).toBe('12m')
    expect(fmtUptime(-5)).toBe('0m')
  })
})

it('fmtMHz rounds', () => expect(fmtMHz(786.431991)).toBe('786 MHz'))

describe('memory formats', () => {
  const GB = 1024 ** 3
  it('gigabytes always with one decimal', () => {
    expect(fmtGiB(4.8 * GB)).toBe('4.8G')
    expect(fmtGiB(11 * GB)).toBe('11.0G')
    expect(fmtGiB(0)).toBe('0.0G')
  })
  it('installed RAM is the reported total rounded up to whole gigabytes', () => {
    expect(fmtInstalledRam(16726118400)).toBe('16 GB') // the CM3588: 15.58 GiB seen by the kernel
    expect(fmtInstalledRam(7.6 * GB)).toBe('8 GB')
    expect(fmtInstalledRam(3.7 * GB)).toBe('4 GB')
    expect(fmtInstalledRam(16 * GB)).toBe('16 GB')
  })
})
