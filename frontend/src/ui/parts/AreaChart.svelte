<!-- A stacked-area chart that fills the free height of its card, with a colour legend below. -->
<script lang="ts">
  import type { Snippet } from 'svelte'
  import { useLayout } from '../layout'

  export interface AreaLegend {
    label: string
    color: string
    value: string
  }

  const { legend, children }: { legend: AreaLegend[]; children: Snippet } = $props()

  const phone = useLayout() === 'mobile'
</script>

<div class="cwrap">{@render children()}</div>
<div class="clegend" class:phone>
  {#each legend as item (item.label)}
    <span class="li"><span class="sw" style:background={item.color}></span>{item.label}<b>{item.value}</b></span>
  {/each}
</div>

<style>
  /* the canvas is absolutely positioned inside, so its bitmap size can't feed back into the layout */
  .cwrap {
    position: relative;
    flex: 1;
    min-height: 96px;
    background: var(--inset);
    border: 1px solid var(--border-2);
    border-radius: 11px;
    overflow: hidden;
  }
  .clegend {
    display: flex;
    flex-wrap: wrap;
    gap: 8px 16px;
    margin-top: 14px;
  }
  .clegend.phone {
    margin-top: 12px;
  }
  .li {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    font-size: 12px;
    color: var(--muted);
  }
  .sw {
    width: 10px;
    height: 10px;
    border-radius: 3px;
    flex: 0 0 auto;
  }
  b {
    font-family: var(--mono);
    color: var(--text);
    font-weight: 500;
  }
</style>
