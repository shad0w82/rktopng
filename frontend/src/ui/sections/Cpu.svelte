<!-- CPU: the cores with usage and frequency, the load of each cluster over time, and where the CPU time goes. -->
<script lang="ts">
  import { drawLines, drawStack } from '../../lib/charts/draw'
  import { palette } from '../../lib/charts/theme'
  import { cpuGovernor, loadBreakdown } from '../../lib/data/derive'
  import { fmtMHz } from '../../lib/data/format'
  import { K } from '../../lib/data/history'
  import { M, seriesKey, val } from '../../lib/data/metrics'
  import { CPU_GROUPS, LINE_COLORS } from '../../lib/data/names'
  import { levelColor, loadLevel } from '../../lib/data/thresholds'
  import { dashboard } from '../../lib/state/dashboard.svelte'
  import AreaChart from '../parts/AreaChart.svelte'
  import BarRow from '../parts/BarRow.svelte'
  import Badge from '../parts/Badge.svelte'
  import Card from '../parts/Card.svelte'
  import ChartTile from '../parts/ChartTile.svelte'
  import RowGroup from '../parts/RowGroup.svelte'
  import SectionHead from '../parts/SectionHead.svelte'
  import Spark from '../parts/Spark.svelte'

  const { live } = dashboard

  const snap = $derived(live.snapshot)
  const governor = $derived(cpuGovernor(snap))
  const breakdown = $derived(loadBreakdown(live.previous, snap))

  const usage = (core: number) => val(snap, M.cpuUsage, { core: String(core) })
  const freq = (core: number) => val(snap, M.cpuFreq, { core: String(core) })
  const history = (core: number) => live.history.get(seriesKey(M.cpuUsage, { core: String(core) }))

  const pct = (v: number | undefined) => (v === undefined ? '–' : `${Math.round(v)}%`)
</script>

<SectionHead label="CPU" icon="cpu" />

<Card title="CPU" span={4}>
  {#snippet badge()}<Badge>governor: <b>{governor ?? 'n/a'}</b></Badge>{/snippet}
  <div>
    {#each CPU_GROUPS as group (group.key)}
      <RowGroup label={group.label}>
        {#each group.cores as core (core)}
          {@const u = usage(core)}
          {@const f = freq(core)}
          <BarRow
            name="core{core}"
            percent={u}
            color={levelColor(loadLevel(u ?? 0))}
            trail={f === undefined ? '—' : fmtMHz(f)}
          />
        {/each}
      </RowGroup>
    {/each}
  </div>
</Card>

<Card title="Load" span={4}>
  <div class="charts">
    {#each CPU_GROUPS as group (group.key)}
      <ChartTile
        name={group.chart}
        legend={group.cores.map((core, i) => ({ label: `core${core}`, color: `var(--${LINE_COLORS[i]})` }))}
      >
        <Spark
          draw={(g, w, h) =>
            drawLines(
              g,
              w,
              h,
              group.cores.map((core, i) => ({ data: history(core), color: palette()[LINE_COLORS[i]] })),
            )}
        />
      </ChartTile>
    {/each}
  </div>
</Card>

<Card title="Load breakdown" span={4}>
  <AreaChart
    legend={[
      { label: 'user', color: 'var(--info)', value: pct(breakdown?.user) },
      { label: 'system', color: 'var(--accent)', value: pct(breakdown?.system) },
      { label: 'iowait', color: 'var(--warn)', value: pct(breakdown?.iowait) },
      { label: 'idle', color: 'var(--faint)', value: pct(breakdown?.idle) },
    ]}
  >
    <Spark
      draw={(g, w, h) => {
        const c = palette()
        const h_ = live.history
        drawStack(g, w, h, [
          { data: h_.get(K.cpuMode('user')), color: c.info + 'e0' },
          { data: h_.get(K.cpuMode('system')), color: c.accent + 'e0' },
          { data: h_.get(K.cpuMode('iowait')), color: c.warn + 'e0' },
          { data: h_.get(K.cpuMode('idle')), color: c.faint + '33', rest: true },
        ])
      }}
    />
  </AreaChart>
</Card>

<style>
  .charts {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
</style>
