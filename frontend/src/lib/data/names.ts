// The backend speaks in driver names; the UI shows short ones (docs/FUNZIONAMENTO.md §6.6).

/** Bus of a disk (label `bus` of rk3588_disk_info) → what the UI prints. */
const BUS_LABEL: Record<string, string> = {
  nvme: 'NVMe',
  sata: 'SATA',
  scsi: 'SCSI',
  usb: 'USB',
  sd: 'SD',
  emmc: 'eMMC',
  virtio: 'virtio',
  other: 'Other',
}
/** Display order of the disk groups. */
export const BUS_ORDER = ['nvme', 'sata', 'scsi', 'usb', 'sd', 'emmc', 'virtio', 'other']

export const busLabel = (bus: string): string => BUS_LABEL[bus] ?? bus
export const busRank = (bus: string): number => {
  const i = BUS_ORDER.indexOf(bus)
  return i < 0 ? BUS_ORDER.length : i
}

/** Thermal zones in display order, with their UI names. */
export const ZONES: ReadonlyArray<{ zone: string; label: string }> = [
  { zone: 'soc', label: 'soc' },
  { zone: 'littlecore', label: 'little' },
  { zone: 'bigcore0', label: 'big_0' },
  { zone: 'bigcore1', label: 'big_1' },
  { zone: 'center', label: 'center' },
  { zone: 'gpu', label: 'gpu' },
  { zone: 'npu', label: 'npu' },
]
export const zoneLabel = (zone: string): string => ZONES.find((z) => z.zone === zone)?.label ?? zone

/** Video engines: the main ones always shown, the others behind the "+" expander. */
export const VPU_MAIN: ReadonlyArray<{ unit: string; label: string }> = [
  { unit: 'enc_core0', label: 'enc0' },
  { unit: 'enc_core1', label: 'enc1' },
  { unit: 'dec_core0', label: 'dec0' },
  { unit: 'dec_core1', label: 'dec1' },
]
export const VPU_EXTRA: ReadonlyArray<{ unit: string; label: string }> = [
  { unit: 'jpeg_enc0', label: 'jpeg0' },
  { unit: 'jpeg_enc1', label: 'jpeg1' },
  { unit: 'jpeg_enc2', label: 'jpeg2' },
  { unit: 'jpeg_enc3', label: 'jpeg3' },
  { unit: 'jpeg_decoder', label: 'jpegd' },
  { unit: 'av1_decoder', label: 'av1d' },
  { unit: 'vdpu', label: 'vdpu' },
  { unit: 'avs_decoder', label: 'avsd' },
  { unit: 'iep', label: 'iep' },
]
export const vpuLabel = (unit: string): string =>
  [...VPU_MAIN, ...VPU_EXTRA].find((u) => u.unit === unit)?.label ?? unit

/** RGA schedulers in display order. */
export const RGA: ReadonlyArray<{ scheduler: string; label: string }> = [
  { scheduler: 'rga3_0', label: 'rga3·0' },
  { scheduler: 'rga3_1', label: 'rga3·1' },
  { scheduler: 'rga2_2', label: 'rga2' },
]
export const rgaLabel = (scheduler: string): string => RGA.find((r) => r.scheduler === scheduler)?.label ?? scheduler

/** Colour of a chart line, by its token name (canvas gets the resolved hex, HTML legends `var(--name)`). */
export type LineColor = 'info' | 'accent' | 'warn' | 'good' | 'orange'
/** Lines of a multi-line chart take these in order. */
export const LINE_COLORS: ReadonlyArray<LineColor> = ['info', 'accent', 'warn', 'good']

/**
 * The RK3588 CPU: 4 × Cortex-A55 (little) and 4 × Cortex-A76 (big) as two pairs.
 * `label` is the vertical tag of the core list, `chart` the title of the Load tile.
 */
export const CPU_GROUPS: ReadonlyArray<{ key: string; label: string; chart: string; cores: readonly number[] }> = [
  { key: 'little', label: 'A55', chart: 'LITTLE', cores: [0, 1, 2, 3] },
  { key: 'big0', label: 'A76', chart: 'BIG_0', cores: [4, 5] },
  { key: 'big1', label: 'A76', chart: 'BIG_1', cores: [6, 7] },
]

/** The three NPU cores (label `core` of rk3588_npu_load_percent). */
export const NPU_CORES: ReadonlyArray<{ core: string; label: string }> = [
  { core: '0', label: 'core0' },
  { core: '1', label: 'core1' },
  { core: '2', label: 'core2' },
]
/** The GPU is a single block; its name fits the 66px name column of the accelerator rows. */
export const GPU_LABEL = 'Mali-G610'
