<!--
  Mobile interface — see mockups/rktopng-mobile.html. The header and the gauges are always
  there; the buttons under them pick ONE section, shown as a column of cards. The cards are
  the ones of the desktop interface (ui/sections), laid out one under the other.
-->
<script lang="ts">
  import { ui } from '../lib/state/ui.svelte'
  import Header from './Header.svelte'
  import { provideLayout } from './layout'
  import TabBar, { type Tab } from './mobile/TabBar.svelte'
  import DiskTiles from './parts/DiskTiles.svelte'
  import QuickLook from './parts/QuickLook.svelte'
  import Accelerators from './sections/Accelerators.svelte'
  import Cpu from './sections/Cpu.svelte'
  import Memory from './sections/Memory.svelte'
  import Network from './sections/Network.svelte'
  import Processes from './sections/Processes.svelte'
  import Storage from './sections/Storage.svelte'
  import Temperature from './sections/Temperature.svelte'

  provideLayout('mobile')

  const tabs: Tab[] = [
    { key: 'overview', label: 'Overview', icon: 'overview' },
    { key: 'cpu', label: 'CPU', icon: 'cpu' },
    { key: 'temp', label: 'Temp', icon: 'temperature' },
    { key: 'accel', label: 'Accel', icon: 'accelerators' },
    { key: 'mem', label: 'Memory', icon: 'memory' },
    { key: 'storage', label: 'Storage', icon: 'storage' },
    { key: 'net', label: 'Network', icon: 'network' },
    { key: 'procs', label: 'Processes', icon: 'processes' },
  ]
  let current = $state('overview')
</script>

<main class="mobile">
  <Header />
  <QuickLook variant="bare" />

  <div class="screen">
    <TabBar {tabs} bind:current />

    <!-- only the chosen section exists: the others draw and fetch nothing -->
    <section class="sec">
      {#if current === 'overview'}
        <DiskTiles />
      {:else if current === 'cpu'}
        <Cpu />
      {:else if current === 'temp'}
        <Temperature />
      {:else if current === 'accel'}
        <Accelerators />
      {:else if current === 'mem'}
        <Memory />
      {:else if current === 'storage'}
        <Storage />
      {:else if current === 'net'}
        <Network />
      {:else if current === 'procs'}
        <Processes />
      {/if}
    </section>

    <p class="foot">
      Mobile interface · <button onclick={() => ui.set('desktop')}>switch to desktop</button>
      {#if ui.override}· <button onclick={() => ui.set(null)}>automatic</button>{/if}
    </p>
  </div>
</main>

<style>
  .mobile {
    max-width: 440px;
    margin: 0 auto;
    background: var(--bg);
    min-height: 100vh;
  }
  .screen {
    padding: 0 14px 26px;
  }
  /* the cards of a section, one under the other */
  .sec {
    margin-top: 12px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .foot {
    text-align: center;
    color: var(--faint);
    font-size: 11px;
    margin: 16px 0 0;
    padding: 0 6px;
  }
  .foot button {
    background: none;
    border: 0;
    padding: 0;
    color: var(--accent);
    cursor: pointer;
    font-size: 11px;
    text-decoration: underline;
  }
</style>
