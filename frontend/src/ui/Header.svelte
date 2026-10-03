<!-- Mobile header: brand and connection state, then uptime / kernel / OS (see mockups/rktopng-mobile.html). -->
<script lang="ts">
  import Brand from './parts/Brand.svelte'
  import HostInfo from './parts/HostInfo.svelte'
  import LiveBadge from './parts/LiveBadge.svelte'
</script>

<header class="hdr">
  <div class="hrow">
    <Brand />
    <div class="live"><LiveBadge /></div>
  </div>
  <div class="facts"><HostInfo /></div>
</header>

<style>
  .hdr {
    padding: 16px 14px 12px;
    background: linear-gradient(180deg, rgba(18, 27, 36, 0.9), rgba(10, 14, 19, 0.6));
    border-bottom: 1px solid var(--border);
  }
  .hrow {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .live {
    margin-left: auto;
    align-self: flex-start;
  }
  .facts {
    margin-top: 12px;
    padding-top: 11px;
    border-top: 1px solid var(--border);
  }
</style>
