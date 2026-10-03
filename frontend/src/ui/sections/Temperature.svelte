<!-- Temperature: the seven thermal zones of the SoC, each with its recent history. -->
<script lang="ts">
  import { zoneTemperatures } from '../../lib/data/derive'
  import { M, seriesKey } from '../../lib/data/metrics'
  import { dashboard } from '../../lib/state/dashboard.svelte'
  import Card from '../parts/Card.svelte'
  import SectionHead from '../parts/SectionHead.svelte'
  import TempTile from '../parts/TempTile.svelte'
  import TileGrid from '../parts/TileGrid.svelte'

  const { live } = dashboard

  const zones = $derived(zoneTemperatures(live.snapshot))
</script>

<SectionHead label="Temperature" icon="temperature" />

<Card title="Thermal zones" span={12}>
  <TileGrid cols={4} smCols={2}>
    {#each zones as z (z.zone)}
      <TempTile label={z.label} celsius={z.celsius} series={seriesKey(M.temp, { zone: z.zone })} />
    {/each}
  </TileGrid>
</Card>
