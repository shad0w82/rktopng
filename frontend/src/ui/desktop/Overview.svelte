<!--
  Overview (desktop): the gauges and the fan on the left, the temperature of every disk
  as plain tiles on the right, both the same height.
-->
<script lang="ts">
  import Card from '../parts/Card.svelte'
  import DiskTiles from '../parts/DiskTiles.svelte'
  import QuickLook from '../parts/QuickLook.svelte'
  import SectionHead from '../parts/SectionHead.svelte'
</script>

<SectionHead label="Overview" icon="overview" />

<Card span={5}>
  <QuickLook />
</Card>

<div class="disks s7">
  <DiskTiles />
</div>

<style>
  .disks {
    display: grid;
    --rows: 1fr 1fr;
  }
</style>
