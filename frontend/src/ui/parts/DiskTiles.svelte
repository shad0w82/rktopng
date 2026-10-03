<!-- The temperature of every disk as plain tiles, three to a row; bus and size under the value. -->
<script lang="ts">
  import { diskTemperatures } from '../../lib/data/derive'
  import { fmtSize } from '../../lib/data/format'
  import { busLabel } from '../../lib/data/names'
  import { levelColor, tempLevel } from '../../lib/data/thresholds'
  import { dashboard } from '../../lib/state/dashboard.svelte'
  import Stat from './Stat.svelte'

  const { info, live } = dashboard

  const disks = $derived(diskTemperatures(info.disks, live.snapshot))
</script>

<div class="grid3">
  {#each disks as d (d.device)}
    <Stat
      variant="disk"
      label={d.device}
      value={d.celsius === undefined ? undefined : Math.round(d.celsius)}
      unit="°C"
      na={d.celsius === undefined}
      stripe={d.celsius === undefined ? 'var(--border-2)' : levelColor(tempLevel(d.celsius))}
    >
      {#snippet sub()}
        <span>{busLabel(d.bus)}</span><span>{d.sizeBytes === undefined ? '' : fmtSize(d.sizeBytes)}</span>
      {/snippet}
    </Stat>
  {/each}
</div>

<style>
  .grid3 {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    grid-template-rows: var(--rows, auto);
    gap: 10px;
  }
</style>
