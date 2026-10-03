<!-- Accelerators: NPU, VPU, RGA and GPU with load and clock, and a Load chart for each. -->
<script lang="ts">
  import { drawLines } from '../../lib/charts/draw'
  import { palette } from '../../lib/charts/theme'
  import { acceleratorDrivers, accelerators, type Engine } from '../../lib/data/accelerators'
  import { fmtMHz } from '../../lib/data/format'
  import { levelColor, loadLevel } from '../../lib/data/thresholds'
  import { dashboard } from '../../lib/state/dashboard.svelte'
  import { useLayout } from '../layout'
  import Badge from '../parts/Badge.svelte'
  import BarRow from '../parts/BarRow.svelte'
  import Card from '../parts/Card.svelte'
  import ChartTile from '../parts/ChartTile.svelte'
  import RowGroup from '../parts/RowGroup.svelte'
  import SectionHead from '../parts/SectionHead.svelte'
  import Spark from '../parts/Spark.svelte'

  const { info, live } = dashboard
  const phone = useLayout() === 'mobile'

  const acc = $derived(accelerators(live.snapshot))
  const drivers = $derived(acceleratorDrivers(info.board))
</script>

{#snippet row(e: Engine)}
  <BarRow
    name={e.label}
    percent={e.load}
    color={levelColor(loadLevel(e.load ?? 0))}
    trail={e.freq === undefined ? '—' : fmtMHz(e.freq)}
    nameCol="66px"
  />
{/snippet}

<SectionHead label="Accelerators" icon="accelerators" />

<Card title="Accelerators" span={4}>
  {#snippet badge()}<Badge>VPU {acc.vpuSessions ?? 'n/a'} sess</Badge>{/snippet}
  <div>
    {#each acc.groups as group (group.key)}
      {#if group.key === 'vpu'}
        <RowGroup label={group.label} expandLabel="show all engines">
          {#each group.engines as e (e.label)}{@render row(e)}{/each}
          {#snippet more()}{#each acc.vpuExtra as e (e.label)}{@render row(e)}{/each}{/snippet}
        </RowGroup>
      {:else}
        <RowGroup label={group.label}>
          {#each group.engines as e (e.label)}{@render row(e)}{/each}
        </RowGroup>
      {/if}
    {/each}
  </div>
</Card>

<Card title="Load" span={8}>
  <div class="charts" class:phone>
    {#each acc.groups as group (group.key)}
      <ChartTile
        name={group.label}
        note={drivers[group.key]?.short}
        noteTitle={drivers[group.key]?.full}
        legend={group.engines.map((e, i) => ({ label: e.label, color: `var(--${group.colors[i]})` }))}
      >
        <Spark
          draw={(g, w, h) =>
            drawLines(
              g,
              w,
              h,
              group.engines.map((e, i) => ({ data: live.history.get(e.series), color: palette()[group.colors[i]] })),
            )}
        />
      </ChartTile>
    {/each}
  </div>
</Card>

<style>
  /* 2×2 tiles that share the card's height, never shorter than 110px */
  .charts {
    flex: 1;
    display: grid;
    grid-template-columns: 1fr 1fr;
    grid-auto-rows: minmax(110px, 1fr);
    gap: 16px;
  }
  /* on the phone: one column of 94px tiles */
  .charts.phone {
    flex: none;
    grid-template-columns: 1fr;
    grid-auto-rows: 94px;
  }
</style>
