<!--
  The system's disks as chips: name, bus and size, and the model. On the desktop each is a button that opens the
  disk's S.M.A.R.T. window (which also has the firmware); a disk that cannot answer S.M.A.R.T. (the eMMC) looks the
  same and says why on hover or focus. `interactive={false}` makes them plain boxes, as on the phone.
-->
<script lang="ts">
  import { fmtSize } from '../../lib/data/format'
  import type { Disk } from '../../lib/data/inventory'
  import { busLabel } from '../../lib/data/names'
  import { hasSmart } from '../../lib/data/storage'
  import { dashboard } from '../../lib/state/dashboard.svelte'

  const { disks, interactive = true, columns = 3 }: { disks: Disk[]; interactive?: boolean; columns?: number } = $props()
</script>

{#snippet face(d: Disk)}
  <span class="l">{d.device}</span>
  <span class="s"><span>{busLabel(d.bus)}</span><span>{d.sizeBytes === undefined ? '' : fmtSize(d.sizeBytes)}</span></span>
  <span class="m" title={d.model}>{d.model || '—'}</span>
{/snippet}

<div class="dsum" style:--cols={columns}>
  {#each disks as d (d.device)}
    {#if !interactive}
      <div class="dchip static">{@render face(d)}</div>
    {:else if hasSmart(d)}
      <button type="button" class="dchip" data-disk={d.device} aria-haspopup="dialog" onclick={(e) => dashboard.smart.open(d.device, e.currentTarget)}>
        {@render face(d)}
      </button>
    {:else}
      <button type="button" class="dchip nosmart" data-disk={d.device} aria-disabled="true" data-tip="S.M.A.R.T. not available">{@render face(d)}</button>
    {/if}
  {/each}
</div>

<style>
  .dsum {
    display: grid;
    grid-template-columns: repeat(var(--cols), minmax(0, 1fr));
    gap: 8px;
    margin-top: 11px;
    padding-top: 13px;
    border-top: 1px solid var(--border-2);
  }
  .dchip {
    appearance: none;
    display: block;
    width: 100%;
    min-width: 0;
    margin: 0;
    font: inherit;
    color: inherit;
    text-align: left;
    background: var(--inset);
    border: 1px solid var(--border);
    border-radius: var(--r-sm);
    padding: 8px 10px;
    transition:
      border-color 0.15s,
      background-color 0.15s;
  }
  @media (max-width: 760px) {
    .dsum {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }
  button.dchip {
    cursor: pointer;
  }
  button.dchip::-moz-focus-inner {
    border: 0;
    padding: 0;
  }
  button.dchip:not(.nosmart):hover {
    border-color: var(--accent);
    background: rgba(52, 208, 189, 0.07);
  }
  button.dchip:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  button.dchip:not(.nosmart):active {
    transform: translateY(1px);
  }

  /* a disk without S.M.A.R.T.: the same tile, no action, and a tooltip that says why */
  .nosmart {
    position: relative;
    cursor: default;
  }
  .nosmart::after {
    content: attr(data-tip);
    position: absolute;
    z-index: 5;
    left: 50%;
    bottom: calc(100% + 6px);
    transform: translateX(-50%);
    width: max-content;
    max-width: 240px;
    padding: 6px 10px;
    background: var(--surface);
    border: 1px solid var(--border-2);
    border-radius: var(--r-sm);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
    color: var(--text);
    font: 400 11.5px/1.4 var(--sans);
    letter-spacing: 0;
    text-transform: none;
    white-space: normal;
    pointer-events: none;
    opacity: 0;
    visibility: hidden;
    transition: opacity 0.12s;
  }
  .nosmart:hover::after,
  .nosmart:focus::after {
    opacity: 1;
    visibility: visible;
  }
  @media (prefers-reduced-motion: reduce) {
    .dchip,
    .nosmart::after {
      transition: none;
    }
  }

  .l {
    display: block;
    font-size: 10.5px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .s {
    display: flex;
    justify-content: space-between;
    gap: 6px;
    margin-top: 2px;
    font-family: var(--mono);
    font-size: 11px;
    color: var(--faint);
    white-space: nowrap;
  }
  .s span:last-child {
    color: var(--text);
  }
  .m {
    display: block;
    margin-top: 5px;
    font-family: var(--mono);
    font-size: 10.5px;
    color: var(--muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
