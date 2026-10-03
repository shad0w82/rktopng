<!-- Desktop top bar, sticky: brand | uptime, kernel, OS | connection (see mockups/rktopng-desktop.html). -->
<script lang="ts">
  import Brand from '../parts/Brand.svelte'
  import HostInfo from '../parts/HostInfo.svelte'
  import LiveBadge from '../parts/LiveBadge.svelte'
</script>

<header class="topbar">
  <div class="in">
    <Brand />
    <div class="facts"><HostInfo /></div>
    <LiveBadge />
  </div>
</header>

<style>
  .topbar {
    position: sticky;
    top: 0;
    z-index: 20;
    background: rgba(10, 14, 19, 0.86);
    backdrop-filter: blur(10px);
    border-bottom: 1px solid var(--border);
  }
  .in {
    max-width: 1280px;
    margin: 0 auto;
    padding: 12px 20px;
    display: flex;
    align-items: center;
    gap: 12px 32px;
    flex-wrap: wrap;
  }
  .facts {
    margin-left: auto;
    --gap: 32px;
    --justify: flex-end;
    --wrap: nowrap;
  }
</style>
