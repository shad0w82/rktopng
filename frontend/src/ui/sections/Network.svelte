<!-- Network: one tile per interface with its download / upload history. -->
<script lang="ts">
  import { networkInterfaces } from '../../lib/data/network'
  import { dashboard } from '../../lib/state/dashboard.svelte'
  import { useLayout } from '../layout'
  import Card from '../parts/Card.svelte'
  import NetTile from '../parts/NetTile.svelte'
  import SectionHead from '../parts/SectionHead.svelte'

  const { live } = dashboard
  const phone = useLayout() === 'mobile'

  const nets = $derived(networkInterfaces(live.snapshot))
</script>

<SectionHead label="Network" icon="network" />

<Card title="Network" span={12}>
  <div class="netwrap" class:phone>
    {#each nets as net (net.iface)}
      <NetTile {net} />
    {/each}
  </div>
</Card>

<style>
  .netwrap {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 14px;
  }
  @media (max-width: 760px) {
    .netwrap {
      grid-template-columns: 1fr;
    }
  }
  .netwrap.phone {
    grid-template-columns: 1fr;
    gap: 12px;
  }
</style>
