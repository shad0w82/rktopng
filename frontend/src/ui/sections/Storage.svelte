<!--
  Storage: capacity of the volumes; temperature of each disk; how busy each disk is; and the
  throughput summed per bus. The disks appear as chips with their model: under the disk rows on the
  desktop (where each opens its S.M.A.R.T. window), under the volumes on the phone.
-->
<script lang="ts">
  import { drawIO } from '../../lib/charts/draw'
  import { palette } from '../../lib/charts/theme'
  import { diskTemperatures, throughputByBus, volumes, zfsSummary } from '../../lib/data/derive'
  import { fmtRate, fmtSize } from '../../lib/data/format'
  import { K } from '../../lib/data/history'
  import { M, seriesKey } from '../../lib/data/metrics'
  import { hasTempSensor, ioGroups, volumeGroups } from '../../lib/data/storage'
  import { diskLevel, levelColor, loadLevel } from '../../lib/data/thresholds'
  import { dashboard } from '../../lib/state/dashboard.svelte'
  import { useLayout } from '../layout'
  import Badge from '../parts/Badge.svelte'
  import BarRow from '../parts/BarRow.svelte'
  import Card from '../parts/Card.svelte'
  import ChartTile from '../parts/ChartTile.svelte'
  import DiskChips from '../parts/DiskChips.svelte'
  import RowGroup from '../parts/RowGroup.svelte'
  import SectionHead from '../parts/SectionHead.svelte'
  import Spark from '../parts/Spark.svelte'
  import TempTile from '../parts/TempTile.svelte'
  import TileGrid from '../parts/TileGrid.svelte'

  const { info, live } = dashboard
  const phone = useLayout() === 'mobile'

  const snap = $derived(live.snapshot)
  const disks = $derived(info.disks)
  const groups = $derived(volumeGroups(volumes(snap), disks))
  const zfs = $derived(zfsSummary(snap))
  const temps = $derived(diskTemperatures(disks.filter(hasTempSensor), snap))
  const io = $derived(ioGroups(disks, snap))
  const bus = $derived(throughputByBus(snap, info.busOf))
</script>

<SectionHead label="Storage" icon="storage" />

{#snippet storageCard()}
<Card title="Storage" span={6}>
  {#snippet badge()}
    {#if zfs}
      <Badge>ZFS: <b style:color={zfs.online === zfs.total ? undefined : 'var(--crit)'}>{zfs.online}/{zfs.total} online</b></Badge>
    {/if}
  {/snippet}
  <div>
    {#each groups as group (group.label)}
      <RowGroup label={group.label} one={group.volumes.length === 1}>
        {#each group.volumes as v (v.mount)}
          <BarRow
            name={v.mount}
            percent={v.pct}
            text={v.pct < 1 ? '<1' : undefined}
            minFill={0.6}
            color={levelColor(diskLevel(v.pct))}
            trail="{fmtSize(v.used)}/{fmtSize(v.size)}"
            nameCol="66px"
            trailCh={9}
          />
        {/each}
      </RowGroup>
    {/each}
  </div>
  {#if phone}
    <DiskChips {disks} interactive={false} columns={2} />
  {/if}
</Card>
{/snippet}

{#snippet tempCard()}
<Card title="Disk temperature" span={6}>
  <TileGrid cols={3} mdCols={3} smCols={2}>
    {#each temps as d (d.device)}
      <TempTile label={d.device} celsius={d.celsius} series={seriesKey(M.diskTemp, { device: d.device })} />
    {/each}
  </TileGrid>
</Card>
{/snippet}

{#snippet ioCard()}
<Card title="Disk I/O" small="(R+W)" span={6}>
  {#snippet badge()}<Badge>{disks.length} disks</Badge>{/snippet}
  <div>
    {#each io as group (group.bus)}
      <RowGroup label={group.label} one={group.rows.length === 1}>
        {#each group.rows as r (r.device)}
          <BarRow
            name={r.device}
            percent={r.busy}
            color={levelColor(loadLevel(r.busy ?? 0))}
            trail={r.rate === undefined ? '—' : fmtRate(r.rate, true)}
            nameCol="66px"
            trailCh={9}
          />
        {/each}
      </RowGroup>
    {/each}
  </div>
  {#if !phone}
    <DiskChips {disks} />
  {/if}
</Card>
{/snippet}

{#snippet throughputCard()}
<Card title="Throughput" small="(ΣR, ΣW)" span={6}>
  <div class="charts">
    {#each io as group (group.bus)}
      <ChartTile
        name={group.label}
        legend={[
          { label: 'R', color: 'var(--info)', value: fmtRate(bus[group.bus]?.read ?? 0) },
          { label: 'W', color: 'var(--accent)', value: fmtRate(bus[group.bus]?.write ?? 0) },
        ]}
      >
        <Spark
          draw={(g, w, h) =>
            drawIO(g, w, h, live.history.get(K.bus(group.bus, 'read')), live.history.get(K.bus(group.bus, 'write')), palette())}
        />
      </ChartTile>
    {/each}
  </div>
</Card>
{/snippet}

<!-- phone: the order of the mockup; desktop: capacity and temperature on the first row, I/O and throughput on the second -->
{#if phone}
  {@render storageCard()}
  {@render ioCard()}
  {@render throughputCard()}
  {@render tempCard()}
{:else}
  {@render storageCard()}
  {@render tempCard()}
  {@render ioCard()}
  {@render throughputCard()}
{/if}

<style>
  /* the tiles share the card's height, never shorter than 94px */
  .charts {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
</style>
