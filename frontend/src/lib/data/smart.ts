// Everything the S.M.A.R.T. window shows, worked out from one /api/smart answer. Pure functions: the
// component only draws. The exporter has already judged the disk (state, levels, reasons) and recognised
// what each attribute means (role); this file words it for people. A figure the drive does not report
// is `null` and reads "n/a": nothing is ever guessed. Rules: docs/smart_status.md.

import type { SmartAttribute, SmartDetail, SmartHealth, SmartLevel } from '../api/types'
import { fmtSize } from './format'

/** The four tones of the UI: good, warning, critical, informative (shown, never counted). */
export type Tone = 'ok' | 'warn' | 'crit' | 'info'

export function tone(level: SmartLevel | undefined): Tone {
  return level === 'fail' ? 'crit' : (level ?? 'ok')
}

/** CSS colour of a tone. */
export const toneColor = (t: Tone): string => `var(--${t === 'ok' ? 'good' : t})`

export function stateLabel(state: SmartHealth['state']): string {
  return { ok: 'OK', warn: 'Warning', fail: 'Failing', unknown: 'Unknown' }[state]
}

export const stateTone = (state: SmartHealth['state']): Tone => (state === 'fail' ? 'crit' : state === 'warn' ? 'warn' : state === 'ok' ? 'ok' : 'info')

// ── numbers ─────────────────────────────────────────────────────────────────────────────────

export const fmtCount = (n: number): string => Math.round(n).toLocaleString('en-US')

/** Decimal units, like the drives' own labels: { value: "6.55", unit: "TB" }. */
export function fmtBytesDec(b: number): { value: string; unit: string } {
  if (b >= 1e12) return { value: (b / 1e12).toFixed(2), unit: 'TB' }
  if (b >= 1e9) return { value: (b / 1e9).toFixed(1), unit: 'GB' }
  return { value: String(Math.round(b / 1e6)), unit: 'MB' }
}

const decText = (b: number): string => {
  const { value, unit } = fmtBytesDec(b)
  return `${value} ${unit}`
}

export function fmtDuration(hours: number): string {
  if (hours >= 13149) return `${(hours / 8766).toFixed(1)} years`
  if (hours >= 48) return `${Math.round(hours / 24)} days`
  return `${hours} h`
}

/** A piece of text, optionally with an explanation shown on hover. */
export interface TextPart {
  text: string
  tip?: string
}

const UNIT_BYTES: Record<string, number> = { MB: 1e6, GB: 1e9, TB: 1e12, GiB: 1024 ** 3, TiB: 1024 ** 4 }

/**
 * "2.00 TB · 1.82 TiB": the same bytes in the decimal unit the maker prints on the box and in the binary one
 * the system reports (and the disk tiles show). Each unit explains itself on hover.
 */
export function capacityParts(bytes: number): TextPart[] {
  const dec = fmtBytesDec(bytes)
  const bin = bytes >= UNIT_BYTES.TiB ? { value: (bytes / UNIT_BYTES.TiB).toFixed(2), unit: 'TiB' } : { value: (bytes / UNIT_BYTES.GiB).toFixed(1), unit: 'GiB' }
  const exact = `This disk: ${fmtCount(bytes)} bytes.`
  return [
    { text: `${dec.value} ${dec.unit}`, tip: `Decimal: 1 ${dec.unit} = ${fmtCount(UNIT_BYTES[dec.unit])} bytes. The size the maker prints on the box. ${exact}` },
    { text: ' · ' },
    { text: `${bin.value} ${bin.unit}`, tip: `Binary: 1 ${bin.unit} = ${fmtCount(UNIT_BYTES[bin.unit])} bytes. The size the system reports, and the one on the disk tiles (${fmtSize(bytes)}). ${exact}` },
  ]
}

export const fmtCapacity = (bytes: number): string => capacityParts(bytes).map((p) => p.text).join('')

/** The line under the window title: "SATA 3.3 · 2.00 TB · firmware SVQ02B6Q". */
export function smartSubtitle(d: SmartDetail): string {
  const i = d.identity
  const kind = d.protocol === 'NVMe' ? i.standard : (i.interface ?? d.protocol)
  return [kind, i.capacity_bytes ? decText(i.capacity_bytes) : '', i.firmware ? `firmware ${i.firmware}` : ''].filter(Boolean).join(' · ')
}

// ── headline cards ──────────────────────────────────────────────────────────────────────────

export interface SmartCard {
  label: string
  /** Null: the drive does not report it ("n/a"). */
  value: string | null
  unit?: string
  sub: string
  tone: Tone
  /** Where the figure comes from (shown on hover). */
  source?: string
}

const na = (label: string): SmartCard => ({ label, value: null, sub: 'not reported by this drive', tone: 'ok' })

const card = (label: string, value: number | undefined, unit: string, sub: string, t: Tone, source?: string): SmartCard =>
  value === undefined ? na(label) : { label, value: fmtCount(value), unit, sub, tone: t, source }

const bytesCard = (label: string, bytes: number | undefined, sub: string, source?: string): SmartCard => {
  if (bytes === undefined) return na(label)
  const { value, unit } = fmtBytesDec(bytes)
  return { label, value, unit, sub, tone: 'info', source }
}

const pct = (n: number, of: number): number => Math.round((100 * n) / of)

/** The eight cards, in the same layout for every protocol: the drive's key figures, then traffic and age. */
export function smartCards(d: SmartDetail): SmartCard[] {
  const v = d.vitals
  const lv = (key: string): Tone => tone(v.levels?.[key])
  const nvme = d.nvme
  const cycles = v.power_cycles

  const wear = card('Wear', v.wear_used_pct, '%', 'rated endurance used', lv('wear'), v.wear_source)
  const temperature = temperatureCard(d)
  const unsafe =
    v.unsafe_shutdowns === undefined
      ? na('Unsafe shutdowns')
      : card('Unsafe shutdowns', v.unsafe_shutdowns, '', cycles ? `of ${fmtCount(cycles)} power cycles (${pct(v.unsafe_shutdowns, cycles)}%)` : 'power lost without a clean shutdown', 'info', v.unsafe_source)
  const age = card('Power-on time', v.power_on_hours, '', '', 'info', 'smartctl')
  if (v.power_on_hours !== undefined) {
    age.unit = 'h'
    age.sub = `≈ ${fmtDuration(v.power_on_hours)}${cycles === undefined ? '' : ` · ${fmtCount(cycles)} power cycles`}`
  }
  const traffic = [
    bytesCard('Data written', v.written_bytes, nvme ? `${fmtCount(nvme.host_writes)} write commands` : 'host writes, whole life', v.written_source),
    bytesCard('Data read', v.read_bytes, nvme ? `${fmtCount(nvme.host_reads)} read commands` : 'host reads, whole life', v.read_source),
    age,
    unsafe,
  ]

  if (nvme) {
    return [
      wear,
      card('Available spare', v.spare_remaining_pct, '%', `threshold ${v.spare_threshold_pct ?? '—'}%`, lv('spare'), 'NVMe health log'),
      temperature,
      card('Media errors', v.media_errors, '', 'unrecovered integrity errors', lv('media_errors'), 'NVMe health log'),
      ...traffic,
    ]
  }
  return [
    wear,
    temperature,
    card('Reallocated sectors', v.reallocated, '', 'bad sectors replaced by spares', lv('reallocated')),
    card('CRC errors', v.crc_errors, '', 'SATA cable / link errors', lv('crc_errors')),
    ...traffic,
  ]
}

function temperatureCard(d: SmartDetail): SmartCard {
  const v = d.vitals
  if (v.temperature_c === undefined) return na('Temperature')
  const peak = v.temp_lifetime_max_c
  const limit = v.temp_limit_c
  let sub = 'no limit reported'
  if (d.nvme && limit !== undefined && v.temp_critical_c !== undefined) sub = `limits ${Math.round(limit)} / ${Math.round(v.temp_critical_c)} °C`
  else if (peak !== undefined && limit !== undefined) sub = `peak ${peak} · limit ${limit} °C`
  else if (limit !== undefined) sub = `limit ${Math.round(limit)} °C`
  const source = d.nvme
    ? 'NVMe composite sensor; limits from the kernel (hwmon)'
    : 'smartctl temperature' + (peak !== undefined ? '. Peak = hottest ever recorded, limit = operating limit, both from the drive’s SCT log; the peak is shown but never counted' : '')
  return { label: 'Temperature', value: fmtCount(v.temperature_c), unit: '°C', sub, tone: tone(v.levels?.temperature), source }
}

/** Shown under the verdict when the report is less trustworthy than usual. */
export function smartNotice(d: SmartDetail): string {
  return d.identity.known_model === false
    ? 'This model is not in smartctl’s drive database: its attribute names may be generic, so only figures from the standard logs or well-known names are shown.'
    : ''
}

// ── identity and logs ───────────────────────────────────────────────────────────────────────

export interface KV {
  label: string
  value: string
  /** Shown faint: an identifier or an absent feature. */
  dim?: boolean
  /** A second line under the value, faint: a note on it ("drive supports …"). */
  sub?: string
  /** `value` in pieces, some with a hover explanation (the plain text is still `value`). */
  parts?: TextPart[]
}

export function smartIdentity(d: SmartDetail): KV[] {
  const i = d.identity
  const rows: Array<KV | null> = [{ label: 'Device', value: `/dev/${d.device}` }]
  if (i.family) rows.push({ label: 'Model family', value: i.family })
  rows.push({ label: 'Model', value: i.model || '—' }, { label: 'Firmware', value: i.firmware || '—' })
  if (i.serial) rows.push({ label: 'Serial number', value: i.serial, dim: true })
  if (i.wwn) rows.push({ label: 'WWN', value: i.wwn, dim: true })
  if (i.capacity_bytes) rows.push({ label: 'Capacity', value: fmtCapacity(i.capacity_bytes), parts: capacityParts(i.capacity_bytes) })
  if (i.standard) rows.push({ label: 'Standard', value: i.standard.replace(/^(ACS-\d+) .*/, '$1') })
  if (i.link_current) {
    const sata = i.interface ? `${i.interface} · ` : ''
    const row: KV = { label: 'Link', value: `${sata}${i.link_current}` }
    // what the link runs at is the value; what the drive could do is a note under it, only when it is more
    if (i.link_max !== undefined && i.link_max !== i.link_current) row.sub = `drive supports ${i.link_max.replace('PCIe ', '')}`
    rows.push(row)
  }
  if (i.block_size) rows.push({ label: 'Block size', value: `${i.block_size} B` })
  const form = [i.form_factor?.replace(/ inches?$/, '″'), i.rotation_rpm === 0 ? 'SSD' : i.rotation_rpm ? `${i.rotation_rpm} rpm` : '', i.trim ? 'TRIM supported' : ''].filter(Boolean)
  if (form.length) rows.push({ label: 'Form factor', value: form.join(' · ') })
  return rows.filter((r): r is KV => r !== null)
}

export function smartLogs(d: SmartDetail): KV[] {
  const n = d.logs.error_entries
  const st = d.logs.self_test
  const rows: KV[] = []
  if (d.protocol === 'NVMe') {
    rows.push({ label: 'Error log entries', value: n === undefined ? 'not reported' : fmtCount(n), dim: n === undefined })
    rows.push({ label: 'Self-test log', value: st ? `${st.count} run` : 'not supported by this drive', dim: !st })
  } else {
    rows.push({ label: 'ATA error log', value: n === undefined ? 'not reported' : n ? `${fmtCount(n)} entries` : 'no entries', dim: n === undefined })
    rows.push({ label: 'Self-tests run', value: !st ? 'not supported by this drive' : st.count ? String(st.count) : 'none yet', dim: !st })
    if (st?.short_minutes !== undefined && st.extended_minutes !== undefined) rows.push({ label: 'Short / extended', value: `≈ ${st.short_minutes} min / ≈ ${st.extended_minutes} min` })
  }
  if (st?.last) rows.push({ label: 'Last self-test', value: `${st.last.type}: ${st.last.status}` })
  return rows
}

/** Only the ATA window says how to schedule self-tests; NVMe drives rarely have them. */
export function smartLogsNote(d: SmartDetail): string {
  return d.protocol !== 'NVMe' && d.logs.self_test ? 'This window only reads. Scheduled self-tests can be set up with smartd (directive -s).' : ''
}

// ── the table ───────────────────────────────────────────────────────────────────────────────

/** What a recognised attribute is, in a sentence; unrecognised ones fall back to a few well-known exact names. */
export const ROLE_INFO: Record<string, string> = {
  reallocated: 'Sectors the drive found bad and replaced with spares. Anything above 0 deserves attention.',
  pending: 'Sectors waiting to be reallocated or re-read.',
  offline_uncorrectable: 'Sectors the background scan could not read.',
  uncorrectable: 'Errors the drive’s ECC could not recover.',
  crc: 'Transmission errors on the SATA link. Above 0 usually means a bad cable or connector.',
  temperature: 'Temperature in °C. Normalized and Worst show the same reading on the drive’s own scale; the all-time peak is in the Temperature card.',
  power_on_hours: 'Total time powered on.',
  power_cycles: 'Number of power-on cycles.',
  wear: 'SSD wear: the normalized value falls from 100 (new) towards 0.',
  written: 'Host data written over the drive’s life.',
  read: 'Host data read over the drive’s life.',
  unsafe_shutdown: 'Power lost without a clean shutdown.',
}
const ECC_INFO = 'Errors corrected by the ECC (normal).'
export const NAME_INFO: Record<string, string> = {
  Program_Fail_Cnt_Total: 'Failed programming (write) operations on flash cells.',
  Erase_Fail_Count_Total: 'Failed erase operations on flash cells.',
  Used_Rsvd_Blk_Cnt_Tot: 'Spare blocks consumed so far.',
  Runtime_Bad_Block: 'Blocks that went bad while the drive was in use.',
  Hardware_ECC_Recovered: ECC_INFO,
  ECC_Error_Rate: ECC_INFO,
}

export interface SmartRow {
  key: string
  tone: Tone
  /** Shown with the "critical" tag and kept by the "Critical only" filter. */
  critical: boolean
  /** ATA only: "5 · 0x05". */
  id?: string
  name: string
  note: string
  /** ATA: normalized / worst / threshold; NVMe: threshold (in `thresh`). */
  normalized?: string
  worst?: string
  thresh: string
  /** The raw value (ATA) or the value (NVMe). */
  value: string
  /** A human reading of the value under it ("≈ 2.3 years"). */
  hint?: string
}

export interface SmartTable {
  kind: 'ata' | 'nvme'
  title: string
  rows: SmartRow[]
}

const hex = (id: number): string => `0x${id.toString(16).toUpperCase().padStart(2, '0')}`

function attributeHint(a: SmartAttribute, cycles: number | undefined): string | undefined {
  switch (a.role) {
    case 'power_on_hours':
      return fmtDuration(a.raw)
    case 'written':
    case 'read':
      return a.bytes === undefined ? undefined : decText(a.bytes)
    case 'temperature':
      return `${a.raw} °C`
    case 'unsafe_shutdown':
      return cycles ? `${pct(a.raw, cycles)}% of power cycles` : undefined
  }
  return undefined
}

function ataRows(d: SmartDetail): SmartRow[] {
  return (d.attributes ?? []).map((a) => ({
    key: String(a.id),
    tone: tone(a.level),
    critical: a.critical,
    id: `${a.id} · ${hex(a.id)}`,
    name: a.name,
    note: (a.role && ROLE_INFO[a.role]) || NAME_INFO[a.name] || '',
    normalized: String(a.value),
    worst: String(a.worst),
    thresh: a.thresh ? String(a.thresh) : '—',
    value: fmtCount(a.raw),
    hint: attributeHint(a, d.vitals.power_cycles),
  }))
}

function nvmeRows(d: SmartDetail): SmartRow[] {
  const n = d.nvme!
  const v = d.vitals
  const lv = (key: string): Tone => tone(n.levels?.[key])
  const row = (key: string, name: string, note: string, value: string, o: { thresh?: string; critical?: boolean; hint?: string; tone?: Tone } = {}): SmartRow => ({
    key,
    name,
    note,
    value,
    thresh: o.thresh ?? '—',
    critical: o.critical ?? false,
    hint: o.hint,
    tone: o.tone ?? lv(key),
  })
  const limits = v.temp_limit_c !== undefined && v.temp_critical_c !== undefined ? `${Math.round(v.temp_limit_c)} / ${Math.round(v.temp_critical_c)} °C` : undefined
  const sensors = (n.sensors ?? []).map((s, i) => `Sensor ${i + 1}: ${s} °C`).join(' · ')
  return [
    row('critical_warning', 'Critical warning', 'Bit field of controller warnings; 0 means none.', `0x${n.critical_warning.toString(16).padStart(2, '0')}`, { thresh: '0x00', critical: true }),
    row('available_spare', 'Available spare', 'Remaining spare capacity, as a percentage.', `${n.available_spare}%`, { thresh: `≥ ${n.available_spare_threshold}%`, critical: true }),
    row('percentage_used', 'Percentage used', 'Vendor estimate of the endurance consumed; can exceed 100%.', `${n.percentage_used}%`, { thresh: '100%', critical: true }),
    row('media_errors', 'Media and data-integrity errors', 'Unrecovered data-integrity errors the controller detected.', fmtCount(n.media_errors), { thresh: '0', critical: true }),
    row('error_log_entries', 'Error log entries', 'Entries in the controller’s error log over its lifetime.', fmtCount(n.error_log_entries), { critical: true }),
    row('temperature', 'Temperature', `Composite temperature.${sensors ? ` ${sensors}.` : ''}`, v.temperature_c === undefined ? '—' : `${v.temperature_c} °C`, { thresh: limits }),
    row('power_on_hours', 'Power-on hours', 'Total time powered on.', fmtCount(n.power_on_hours), { hint: `≈ ${fmtDuration(n.power_on_hours)}` }),
    row('power_cycles', 'Power cycles', 'Number of power-on cycles.', fmtCount(n.power_cycles)),
    row('unsafe_shutdowns', 'Unsafe shutdowns', 'Shutdowns without a shutdown notification (power loss or abrupt power-off).', fmtCount(n.unsafe_shutdowns), {
      hint: n.power_cycles ? `${pct(n.unsafe_shutdowns, n.power_cycles)}% of cycles` : undefined,
      tone: 'info',
    }),
    row('data_units_read', 'Data units read', 'Host reads, in units of 512,000 bytes.', fmtCount(n.data_units_read), { hint: decText(n.data_units_read * 512000) }),
    row('data_units_written', 'Data units written', 'Host writes, in units of 512,000 bytes.', fmtCount(n.data_units_written), { hint: decText(n.data_units_written * 512000) }),
    row('host_reads', 'Host read commands', 'Completed read commands.', fmtCount(n.host_reads)),
    row('host_writes', 'Host write commands', 'Completed write commands.', fmtCount(n.host_writes)),
    row('controller_busy', 'Controller busy time', 'Time the controller spent processing I/O commands.', `${fmtCount(n.controller_busy_minutes)} min`),
    row('warning_temp', 'Warning temperature time', 'Time spent above the warning temperature.', `${fmtCount(n.warning_temp_minutes)} min`),
    row('critical_temp', 'Critical temperature time', 'Time spent above the critical temperature.', `${fmtCount(n.critical_temp_minutes)} min`),
  ]
}

export function smartTable(d: SmartDetail): SmartTable {
  return d.nvme ? { kind: 'nvme', title: 'NVMe health log', rows: nvmeRows(d) } : { kind: 'ata', title: 'S.M.A.R.T. attributes', rows: ataRows(d) }
}

// ── the drive's own temperature log ─────────────────────────────────────────────────────────

export interface TempChart {
  /** SVG points for the line and for the filled area, in a 800 x 96 box. */
  line: string
  area: string
  min: number
  max: number
  now: number
  /** How far back the log reaches, e.g. "21 h". */
  span: string
  intervalMinutes: number
}

export const CHART_W = 800
export const CHART_H = 96

export function tempChart(d: SmartDetail): TempChart | null {
  const h = d.temp_history
  if (!h || h.samples.length < 2) return null
  const s = h.samples
  const lo = Math.min(...s) - 1
  const hi = Math.max(...s) + 1
  const pts = s.map((v, i) => `${((i / (s.length - 1)) * CHART_W).toFixed(1)},${(CHART_H - 4 - ((v - lo) / (hi - lo)) * (CHART_H - 12)).toFixed(1)}`)
  const hours = Math.round((s.length * h.interval_minutes) / 60)
  return {
    line: pts.join(' '),
    area: `0,${CHART_H} ${pts.join(' ')} ${CHART_W},${CHART_H}`,
    min: Math.min(...s),
    max: Math.max(...s),
    now: s[s.length - 1],
    span: `${hours} h`,
    intervalMinutes: h.interval_minutes,
  }
}

/** Why a disk could not be read, in words. */
export function unavailableText(d: SmartDetail): string {
  switch (d.reason) {
    case 'standby':
      return 'The disk is in standby. It was not woken to read S.M.A.R.T.; use it and open this window again.'
    case 'permission':
      return 'The exporter has no permission to read this disk. S.M.A.R.T. needs root (or CAP_SYS_RAWIO).'
    case 'smartctl_missing':
      return 'smartctl is not installed on the system (package smartmontools).'
    case 'disabled':
      return 'S.M.A.R.T. is turned off on this exporter (RKTOP_SMART).'
    case 'unsupported':
      return 'This disk does not answer S.M.A.R.T. queries (an USB adapter may not pass them through).'
  }
  return d.message || 'The disk could not be read.'
}
