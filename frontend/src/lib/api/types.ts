// Shapes of the exporter's JSON (see docs/FUNZIONAMENTO.md, section 4).

/** One metric point: a value plus its labels (absent for metrics without labels). */
export interface Sample {
  labels?: Record<string, string>
  value: number
}

/** /api/info, /api/stream and /api/stats all carry this: metrics grouped by metric name. */
export interface Snapshot {
  /** Unix milliseconds of the reading. */
  timestamp: number
  metrics: Record<string, Sample[]>
}

export type ProcessSort = 'cpu' | 'mem' | 'pid' | 'name' | 'user' | 'threads'

export interface ProcessRow {
  pid: number
  name: string
  user: string
  state: string
  cmd: string
  /** CPU percent of one core (can exceed 100 for multi-threaded processes). */
  cpu: number
  mem_pct: number
  mem_bytes: number
  threads: number
}

/** /api/processes */
export interface ProcessList {
  timestamp: number
  sort: string
  /** The order was flipped (the other end of the list). */
  reverse?: boolean
  /** All processes on the system, not only the returned rows (the "N task" badge). */
  total: number
  processes: ProcessRow[]
}

// ── /api/smart/<device>: the S.M.A.R.T. report of one disk (see docs/smart_status.md) ────────────

/** Level of a figure. "info" is shown but never counted (e.g. unsafe shutdowns). */
export type SmartLevel = 'ok' | 'warn' | 'fail' | 'info'

export interface SmartReason {
  level: 'warn' | 'fail'
  code: string
  text: string
}

export interface SmartIdentity {
  model?: string
  family?: string
  firmware?: string
  serial?: string
  wwn?: string
  capacity_bytes?: number
  block_size?: number
  /** 0 = solid state. */
  rotation_rpm?: number
  form_factor?: string
  standard?: string
  interface?: string
  link_current?: string
  link_max?: string
  trim?: boolean
  /** ATA: the model is in smartctl's drive database (its attribute names are reliable). */
  known_model?: boolean
}

export interface SmartHealth {
  /** "unknown": the disk could not be read. */
  state: 'ok' | 'warn' | 'fail' | 'unknown'
  /** The drive's own overall self-assessment. */
  passed?: boolean
  reasons?: SmartReason[]
}

/** Headline figures; a figure the drive does not report is absent (never 0). */
export interface SmartVitals {
  temperature_c?: number
  temp_lifetime_max_c?: number
  temp_lifetime_min_c?: number
  /** Operating limit (ATA SCT) or warning limit (NVMe). */
  temp_limit_c?: number
  temp_critical_c?: number
  power_on_hours?: number
  power_cycles?: number
  wear_used_pct?: number
  wear_source?: string
  spare_remaining_pct?: number
  spare_threshold_pct?: number
  written_bytes?: number
  written_source?: string
  read_bytes?: number
  read_source?: string
  unsafe_shutdowns?: number
  unsafe_source?: string
  reallocated?: number
  pending?: number
  offline_uncorrectable?: number
  uncorrectable?: number
  crc_errors?: number
  media_errors?: number
  levels: Record<string, SmartLevel>
}

export interface SmartAttribute {
  id: number
  name: string
  /** Normalized value (higher is better). */
  value: number
  worst: number
  /** 0 = none. */
  thresh: number
  raw: number
  raw_text: string
  /** Host-traffic attributes (written / read): the raw value converted to bytes. */
  bytes?: number
  prefail: boolean
  when_failed?: 'now' | 'past'
  /** What smartctl's name means (reallocated, crc, wear, written, …); absent when not recognised. */
  role?: string
  critical: boolean
  level: SmartLevel
}

export interface SmartNVMe {
  critical_warning: number
  available_spare: number
  available_spare_threshold: number
  percentage_used: number
  data_units_read: number
  data_units_written: number
  host_reads: number
  host_writes: number
  controller_busy_minutes: number
  power_cycles: number
  power_on_hours: number
  unsafe_shutdowns: number
  media_errors: number
  error_log_entries: number
  warning_temp_minutes: number
  critical_temp_minutes: number
  sensors?: number[]
  levels: Record<string, SmartLevel>
}

export interface SmartSelfTest {
  count: number
  short_minutes?: number
  extended_minutes?: number
  last?: { type: string; status: string; passed: boolean; lifetime_hours?: number }
}

export interface SmartDetail {
  device: string
  /** false: nothing could be read, see `reason`. */
  available: boolean
  reason?: 'standby' | 'permission' | 'unsupported' | 'disabled' | 'smartctl_missing' | 'failed'
  message?: string
  read_at: number
  smartctl?: string
  protocol?: 'ATA' | 'NVMe' | 'SCSI'
  identity: SmartIdentity
  health: SmartHealth
  vitals: SmartVitals
  attributes?: SmartAttribute[]
  nvme?: SmartNVMe
  logs: { error_entries?: number; self_test?: SmartSelfTest }
  /** The drive's own temperature log (ATA SCT), oldest first. */
  temp_history?: { interval_minutes: number; samples: number[] }
}
