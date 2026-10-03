import { describe, expect, it } from 'vitest'
import type { SmartDetail } from '../api/types'
import { lexarNvme, samsungSata } from '../test/smart-fixtures'
import {
  capacityParts,
  CHART_H,
  CHART_W,
  fmtBytesDec,
  fmtCapacity,
  fmtDuration,
  smartCards,
  smartIdentity,
  smartLogs,
  smartLogsNote,
  smartNotice,
  smartSubtitle,
  smartTable,
  stateLabel,
  tempChart,
  tone,
  toneColor,
  unavailableText,
} from './smart'

const clone = (d: SmartDetail): SmartDetail => structuredClone(d)
const card = (cards: ReturnType<typeof smartCards>, label: string) => cards.find((c) => c.label === label)!

describe('tones and labels', () => {
  it('maps the exporter levels onto the UI tones and colours', () => {
    expect(tone('fail')).toBe('crit')
    expect(tone('warn')).toBe('warn')
    expect(tone('info')).toBe('info')
    expect(tone('ok')).toBe('ok')
    expect(tone(undefined)).toBe('ok')
    expect(toneColor('ok')).toBe('var(--good)')
    expect(toneColor('crit')).toBe('var(--crit)')
    expect([stateLabel('ok'), stateLabel('warn'), stateLabel('fail'), stateLabel('unknown')]).toEqual(['OK', 'Warning', 'Failing', 'Unknown'])
  })
})

describe('number formats', () => {
  it('writes sizes in decimal units and durations in the largest sensible unit', () => {
    expect(fmtBytesDec(6.548e12)).toEqual({ value: '6.55', unit: 'TB' })
    expect(fmtBytesDec(37.5e9)).toEqual({ value: '37.5', unit: 'GB' })
    expect(fmtBytesDec(250e6)).toEqual({ value: '250', unit: 'MB' })
    expect(fmtDuration(20)).toBe('20 h')
    expect(fmtDuration(7247)).toBe('302 days')
    expect(fmtDuration(36635)).toBe('4.2 years')
    expect(fmtCapacity(2000398934016)).toBe('2.00 TB · 1.82 TiB')
    expect(fmtCapacity(1024209543168)).toBe('1.02 TB · 953.9 GiB')
  })
})

describe('capacity in two units', () => {
  it('says both sizes, each with its own explanation', () => {
    const [dec, dot, bin] = capacityParts(2000398934016)
    expect([dec.text, dot.text, bin.text]).toEqual(['2.00 TB', ' · ', '1.82 TiB'])
    expect(dot.tip).toBeUndefined()
    expect(dec.tip).toContain('1 TB = 1,000,000,000,000 bytes')
    expect(dec.tip).toContain('maker prints on the box')
    expect(bin.tip).toContain('1 TiB = 1,099,511,627,776 bytes')
    expect(bin.tip).toContain('disk tiles (1.8T)') // the tile shows the same disk as 1.8T
    for (const p of [dec, bin]) expect(p.tip).toContain('This disk: 2,000,398,934,016 bytes.')
  })

  it('uses GB and GiB below a terabyte', () => {
    const [dec, , bin] = capacityParts(1024209543168)
    expect([dec.text, bin.text]).toEqual(['1.02 TB', '953.9 GiB'])
    expect(bin.tip).toContain('1 GiB = 1,073,741,824 bytes')
    expect(capacityParts(256060514304).map((p) => p.text).join('')).toBe('256.1 GB · 238.5 GiB')
    expect(capacityParts(256060514304)[0].tip).toContain('1 GB = 1,000,000,000 bytes')
  })

  it('the identity row carries the pieces and the same plain text', () => {
    const row = smartIdentity(samsungSata).find((r) => r.label === 'Capacity')!
    expect(row.value).toBe('2.00 TB · 1.82 TiB')
    expect(row.parts!.map((p) => p.text).join('')).toBe(row.value)
    expect(row.parts!.filter((p) => p.tip)).toHaveLength(2)
  })
})

describe('cards', () => {
  it('SATA: wear, temperature with its peak and limit, errors, traffic and age — all from the standard logs', () => {
    const c = smartCards(samsungSata)
    expect(c.map((x) => x.label)).toEqual(['Wear', 'Temperature', 'Reallocated sectors', 'CRC errors', 'Data written', 'Data read', 'Power-on time', 'Unsafe shutdowns'])
    expect(card(c, 'Wear')).toMatchObject({ value: '0', unit: '%', tone: 'ok', source: 'device statistics' })
    expect(card(c, 'Temperature').sub).toBe('peak 84 · limit 70 °C') // a past peak above the limit is shown, not counted
    expect(card(c, 'Temperature').tone).toBe('ok')
    expect(card(c, 'Data written')).toMatchObject({ value: '4.50', unit: 'TB', tone: 'info' })
    expect(card(c, 'Unsafe shutdowns')).toMatchObject({ value: '99', sub: 'of 107 power cycles (93%)', tone: 'info' })
    expect(card(c, 'Unsafe shutdowns').source).toContain('POR_Recovery_Count')
    expect(card(c, 'Power-on time')).toMatchObject({ unit: 'h', sub: '≈ 2.3 years · 107 power cycles' })
  })

  it('NVMe: spare and media errors take the place of the SATA counters, limits come from the kernel', () => {
    const c = smartCards(lexarNvme)
    expect(c.map((x) => x.label)).toEqual(['Wear', 'Available spare', 'Temperature', 'Media errors', 'Data written', 'Data read', 'Power-on time', 'Unsafe shutdowns'])
    expect(card(c, 'Available spare')).toMatchObject({ value: '100', unit: '%', sub: 'threshold 10%' })
    expect(card(c, 'Temperature').sub).toBe('limits 90 / 95 °C')
    expect(card(c, 'Data written').sub).toMatch(/write commands$/)
    expect(card(c, 'Unsafe shutdowns').sub).toMatch(/^of 189 power cycles \(5\d%\)$/)
  })

  it('a figure the drive does not report is n/a, never a made-up zero', () => {
    const d = clone(samsungSata)
    for (const k of ['wear_used_pct', 'written_bytes', 'read_bytes', 'unsafe_shutdowns', 'reallocated', 'crc_errors', 'temp_lifetime_max_c', 'temp_limit_c'] as const) delete d.vitals[k]
    const c = smartCards(d)
    for (const label of ['Wear', 'Data written', 'Data read', 'Unsafe shutdowns', 'Reallocated sectors', 'CRC errors']) {
      expect(card(c, label)).toMatchObject({ value: null, sub: 'not reported by this drive' })
    }
    expect(card(c, 'Temperature').sub).toBe('no limit reported')
    expect(card(c, 'Power-on time').value).toBe('20,396')
  })

  it('carries the exporter levels to the card colours', () => {
    const d = clone(samsungSata)
    d.vitals.reallocated = 8
    d.vitals.levels.reallocated = 'warn'
    d.vitals.wear_used_pct = 100
    d.vitals.levels.wear = 'fail'
    const c = smartCards(d)
    expect(card(c, 'Reallocated sectors')).toMatchObject({ value: '8', tone: 'warn' })
    expect(card(c, 'Wear')).toMatchObject({ value: '100', tone: 'crit' })
  })
})

describe('notice, identity and logs', () => {
  it('the line under the title names the interface, the size and the firmware', () => {
    expect(smartSubtitle(samsungSata)).toBe('SATA 3.3 · 2.00 TB · firmware SVQ02B6Q')
    expect(smartSubtitle(lexarNvme)).toBe('NVMe 1.4 · 1.02 TB · firmware 11296')
    const d = clone(samsungSata)
    delete d.identity.interface
    delete d.identity.firmware
    expect(smartSubtitle(d)).toBe('ATA · 2.00 TB')
  })

  it('warns only when the model is missing from smartctl’s database', () => {
    expect(smartNotice(samsungSata)).toBe('')
    const d = clone(samsungSata)
    d.identity.known_model = false
    expect(smartNotice(d)).toMatch(/not in smartctl/)
    delete d.identity.known_model // NVMe has no such field
    expect(smartNotice(d)).toBe('')
  })

  it('SATA identity: family, serial, link with its maximum, form factor', () => {
    const rows = Object.fromEntries(smartIdentity(samsungSata).map((r) => [r.label, r]))
    expect(rows['Device'].value).toBe('/dev/sda')
    expect(rows['Model family'].value).toBe('Samsung based SSDs')
    expect(rows['Serial number']).toMatchObject({ value: 'TESTSERIAL0001', dim: true })
    expect(rows['Capacity'].value).toBe('2.00 TB · 1.82 TiB')
    expect(rows['Standard'].value).toBe('ACS-4')
    expect(rows['Link']).toMatchObject({ value: 'SATA 3.3 · 6.0 Gb/s' })
    expect(rows['Link'].sub).toBeUndefined() // running at the most the drive supports: nothing to add
    expect(rows['Form factor'].value).toBe('2.5″ · SSD · TRIM supported')
  })

  it('a SATA link below the drive’s maximum gets the same second line', () => {
    const d = clone(samsungSata)
    d.identity.link_current = '3.0 Gb/s'
    const link = smartIdentity(d).find((r) => r.label === 'Link')!
    expect(link).toMatchObject({ value: 'SATA 3.3 · 3.0 Gb/s', sub: 'drive supports 6.0 Gb/s' })
  })

  it('NVMe identity says when the slot gives the drive less than it supports', () => {
    const rows = Object.fromEntries(smartIdentity(lexarNvme).map((r) => [r.label, r]))
    expect(rows['Standard'].value).toBe('NVMe 1.4')
    expect(rows['Link']).toMatchObject({ value: 'PCIe Gen3 ×1 · 8.0 GT/s', sub: 'drive supports Gen4 ×4 · 16.0 GT/s' })
    expect(rows['Model family']).toBeUndefined()
    expect(rows['WWN']).toBeUndefined()
  })

  it('logs: SATA lists its self-test capacity, NVMe says it has no self-test log', () => {
    const sata = Object.fromEntries(smartLogs(samsungSata).map((r) => [r.label, r.value]))
    expect(sata).toEqual({ 'ATA error log': 'no entries', 'Self-tests run': 'none yet', 'Short / extended': '≈ 2 min / ≈ 160 min' })
    expect(smartLogsNote(samsungSata)).toMatch(/smartd/)
    const nvme = Object.fromEntries(smartLogs(lexarNvme).map((r) => [r.label, r.value]))
    expect(nvme).toEqual({ 'Error log entries': '0', 'Self-test log': 'not supported by this drive' })
    expect(smartLogsNote(lexarNvme)).toBe('')

    const d = clone(samsungSata)
    d.logs = { error_entries: 2, self_test: { count: 1, last: { type: 'Extended offline', status: 'Completed: read failure', passed: false } } }
    const rows = Object.fromEntries(smartLogs(d).map((r) => [r.label, r.value]))
    expect(rows['ATA error log']).toBe('2 entries')
    expect(rows['Last self-test']).toBe('Extended offline: Completed: read failure')
    delete d.logs.self_test
    expect(smartLogs(d).find((r) => r.label === 'Self-tests run')).toMatchObject({ value: 'not supported by this drive', dim: true })
  })
})

describe('table', () => {
  it('SATA rows keep smartctl’s own names and mark the critical ones', () => {
    const t = smartTable(samsungSata)
    expect(t.kind).toBe('ata')
    expect(t.rows).toHaveLength(14)
    expect(t.rows.filter((r) => r.critical).map((r) => r.key)).toEqual(['5', '177', '179', '183', '187', '199'])
    const wear = t.rows.find((r) => r.key === '177')!
    expect(wear).toMatchObject({ id: '177 · 0xB1', name: 'Wear_Leveling_Count', normalized: '99', worst: '99', thresh: '—', tone: 'ok' })
    expect(wear.note).toMatch(/wear/i)
    expect(t.rows.find((r) => r.key === '5')).toMatchObject({ thresh: '10', note: expect.stringMatching(/replaced with spares/) })
  })

  it('describes by role, falls back to a few exact names, otherwise says nothing', () => {
    const t = smartTable(samsungSata)
    expect(t.rows.find((r) => r.key === '181')!.note).toMatch(/programming/)
    const d = clone(samsungSata)
    d.attributes![0] = { ...d.attributes![0], name: 'Unknown_Attribute', role: undefined }
    expect(smartTable(d).rows[0].note).toBe('')
  })

  it('shows a human reading under the raw value, by role', () => {
    const t = smartTable(samsungSata)
    expect(t.rows.find((r) => r.key === '9')!.hint).toBe('2.3 years')
    expect(t.rows.find((r) => r.key === '241')!.hint).toBe('4.50 TB')
    expect(t.rows.find((r) => r.key === '235')).toMatchObject({ tone: 'info', hint: '93% of power cycles' })
    expect(t.rows.find((r) => r.key === '190')!.hint).toBe('34 °C')
    expect(t.rows.find((r) => r.key === '12')!.hint).toBeUndefined()
  })

  it('NVMe rows: thresholds, the five critical fields, unsafe shutdowns informational', () => {
    const t = smartTable(lexarNvme)
    expect(t.kind).toBe('nvme')
    expect(t.rows).toHaveLength(16)
    expect(t.rows.filter((r) => r.critical).map((r) => r.key)).toEqual(['critical_warning', 'available_spare', 'percentage_used', 'media_errors', 'error_log_entries'])
    expect(t.rows.find((r) => r.key === 'available_spare')).toMatchObject({ value: '100%', thresh: '≥ 10%' })
    expect(t.rows.find((r) => r.key === 'temperature')).toMatchObject({ thresh: '90 / 95 °C' })
    expect(t.rows.find((r) => r.key === 'temperature')!.note).toMatch(/Sensor 1: \d+ °C · Sensor 2: \d+ °C/)
    expect(t.rows.find((r) => r.key === 'unsafe_shutdowns')).toMatchObject({ tone: 'info', hint: expect.stringMatching(/% of cycles$/) })
    expect(t.rows.find((r) => r.key === 'data_units_written')!.hint).toMatch(/TB$/)
  })

  it('NVMe rows take their tone from the exporter’s levels', () => {
    const d = clone(lexarNvme)
    d.nvme!.levels = { media_errors: 'warn', available_spare: 'fail' }
    const t = smartTable(d)
    expect(t.rows.find((r) => r.key === 'media_errors')!.tone).toBe('warn')
    expect(t.rows.find((r) => r.key === 'available_spare')!.tone).toBe('crit')
    expect(t.rows.find((r) => r.key === 'power_cycles')!.tone).toBe('ok')
  })
})

describe('temperature chart', () => {
  it('draws the drive’s own log across the box, scaled to its range', () => {
    const c = tempChart(samsungSata)!
    const pts = c.line.split(' ').map((p) => p.split(',').map(Number))
    expect(pts).toHaveLength(24)
    expect(pts[0][0]).toBe(0)
    expect(pts[23][0]).toBeCloseTo(CHART_W, 5)
    for (const [, y] of pts) expect(y).toBeGreaterThan(0), expect(y).toBeLessThan(CHART_H)
    expect(c.area.startsWith(`0,${CHART_H} `)).toBe(true)
    expect(c.area.endsWith(` ${CHART_W},${CHART_H}`)).toBe(true)
    expect(c.now).toBe(samsungSata.temp_history!.samples.at(-1))
    expect(c.span).toBe('4 h') // 24 samples of 10 minutes (the fixture is cut; the real log is 128)
  })

  it('a hotter sample is drawn higher', () => {
    const d = clone(samsungSata)
    d.temp_history = { interval_minutes: 10, samples: [30, 40, 30] }
    const pts = tempChart(d)!.line.split(' ').map((p) => Number(p.split(',')[1]))
    expect(pts[1]).toBeLessThan(pts[0])
    expect(pts[0]).toBeCloseTo(pts[2], 5)
  })

  it('has nothing to draw without a log (NVMe) or with a single sample', () => {
    expect(tempChart(lexarNvme)).toBeNull()
    const d = clone(samsungSata)
    d.temp_history = { interval_minutes: 10, samples: [33] }
    expect(tempChart(d)).toBeNull()
  })
})

describe('unavailable disks', () => {
  it('explains why in words', () => {
    const base = { device: 'sda', available: false, read_at: 1, identity: {}, health: { state: 'unknown' as const }, vitals: { levels: {} }, logs: {} }
    expect(unavailableText({ ...base, reason: 'standby' })).toMatch(/standby/)
    expect(unavailableText({ ...base, reason: 'permission' })).toMatch(/root/)
    expect(unavailableText({ ...base, reason: 'smartctl_missing' })).toMatch(/smartmontools/)
    expect(unavailableText({ ...base, reason: 'disabled' })).toMatch(/RKTOP_SMART/)
    expect(unavailableText({ ...base, reason: 'unsupported' })).toMatch(/USB/)
    expect(unavailableText({ ...base, reason: 'failed', message: 'boom' })).toBe('boom')
  })
})
