<!-- Desktop interface: every section at once on a 12-column grid, separated by a rule with the section's icon — see mockups/rktopng-desktop.html. -->
<script lang="ts">
  import { ui } from '../lib/state/ui.svelte'
  import { provideLayout } from './layout'
  import Accelerators from './sections/Accelerators.svelte'
  import Cpu from './sections/Cpu.svelte'
  import Memory from './sections/Memory.svelte'
  import Network from './sections/Network.svelte'
  import Overview from './desktop/Overview.svelte'
  import Processes from './sections/Processes.svelte'
  import SmartWindow from './smart/SmartWindow.svelte'
  import Storage from './sections/Storage.svelte'
  import Temperature from './sections/Temperature.svelte'
  import TopBar from './desktop/TopBar.svelte'

  provideLayout('desktop')
</script>

<TopBar />
<main class="wrap">
  <div class="grid">
    <Overview />
    <Cpu />
    <Temperature />
    <Accelerators />
    <Memory />
    <Storage />
    <Network />
    <Processes />
  </div>
  <p class="foot">
    Desktop interface · <button onclick={() => ui.set('mobile')}>switch to mobile</button>
    {#if ui.override}· <button onclick={() => ui.set(null)}>automatic</button>{/if}
  </p>
</main>
<SmartWindow />

<style>
  .wrap {
    max-width: 1280px;
    margin: 0 auto;
    padding: 0 20px 36px;
  }
  /* 12 columns; every row of cards sums to 12 */
  .grid {
    display: grid;
    grid-template-columns: repeat(12, minmax(0, 1fr));
    gap: 14px;
    margin-top: 16px;
  }
  .grid :global(.s4) { grid-column: span 4; }
  .grid :global(.s5) { grid-column: span 5; }
  .grid :global(.s6) { grid-column: span 6; }
  .grid :global(.s7) { grid-column: span 7; }
  .grid :global(.s8) { grid-column: span 8; }
  .grid :global(.s12) { grid-column: span 12; }
  @media (max-width: 1100px) {
    .grid :global(.s4),
    .grid :global(.s5),
    .grid :global(.s7),
    .grid :global(.s8) {
      grid-column: span 6;
    }
  }
  @media (max-width: 760px) {
    .grid :global(.s4),
    .grid :global(.s5),
    .grid :global(.s6),
    .grid :global(.s7),
    .grid :global(.s8) {
      grid-column: span 12;
    }
  }
  .foot {
    text-align: center;
    color: var(--faint);
    font-size: 11px;
    margin: 16px 0 0;
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
