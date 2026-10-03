<!-- Memory: what is available, swap, the memory controller and CMA; and how the RAM splits over time. -->
<script lang="ts">
  import { drawStack } from '../../lib/charts/draw'
  import { palette } from '../../lib/charts/theme'
  import { cmaUsage, memBreakdown, swapUsage } from '../../lib/data/derive'
  import { fmtGiB, fmtInstalledRam, fmtMHz, fmtSize } from '../../lib/data/format'
  import { K } from '../../lib/data/history'
  import { M, val } from '../../lib/data/metrics'
  import { levelColor, loadLevel } from '../../lib/data/thresholds'
  import { dashboard } from '../../lib/state/dashboard.svelte'
  import AreaChart from '../parts/AreaChart.svelte'
  import Badge from '../parts/Badge.svelte'
  import BarRow from '../parts/BarRow.svelte'
  import Card from '../parts/Card.svelte'
  import SectionHead from '../parts/SectionHead.svelte'
  import Spark from '../parts/Spark.svelte'

  const { live } = dashboard

  const snap = $derived(live.snapshot)
  const mem = $derived(memBreakdown(snap))
  const swap = $derived(swapUsage(snap))
  const cma = $derived(cmaUsage(snap))
  const ddrLoad = $derived(val(snap, M.ddrLoad))
  const ddrFreq = $derived(val(snap, M.ddrFreq))

  const color = (pct: number | undefined) => levelColor(loadLevel(pct ?? 0))
  const used = (u: { used: number; total: number } | undefined) => (u ? `${fmtSize(u.used)}/${fmtSize(u.total)}` : '—')
  const gib = (bytes: number | undefined) => (bytes === undefined ? '–' : fmtGiB(bytes))
</script>

<SectionHead label="Memory" icon="memory" />

<Card title="Memory" span={6}>
  {#snippet badge()}
    <Badge>Avail: <b>{gib(mem?.available)}</b> - Total: <b>{mem ? fmtInstalledRam(mem.total) : 'n/a'}</b></Badge>
  {/snippet}
  <div>
    <BarRow name="Swap" percent={swap?.pct} color={color(swap?.pct)} trail={used(swap)} nameCol="62px" trailCh={9} pctWidth="4ch" />
    <BarRow
      name="DDR Ctrl"
      percent={ddrLoad}
      color={color(ddrLoad)}
      trail={ddrFreq === undefined ? '—' : fmtMHz(ddrFreq)}
      nameCol="62px"
      trailCh={9}
      pctWidth="4ch"
    />
    <BarRow name="CMA" percent={cma?.pct} color={color(cma?.pct)} trail={used(cma)} nameCol="62px" trailCh={9} pctWidth="4ch" />
  </div>
</Card>

<Card title="Memory breakdown" span={6}>
  <AreaChart
    legend={[
      { label: 'used', color: 'var(--info)', value: gib(mem?.used) },
      { label: 'cache', color: 'var(--accent)', value: gib(mem?.cache) },
      { label: 'free', color: 'var(--faint)', value: gib(mem?.free) },
    ]}
  >
    <Spark
      draw={(g, w, h) => {
        const c = palette()
        drawStack(g, w, h, [
          { data: live.history.get(K.ramUsed), color: c.info + 'e0' },
          { data: live.history.get(K.ramCache), color: c.accent + 'e0' },
          { data: [], color: c.faint + '33', rest: true },
        ])
      }}
    />
  </AreaChart>
</Card>
