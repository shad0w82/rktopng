<!--
  A rounded chart tile: name top-left, legend top-right (in the band the chart keeps
  free), the chart itself behind (pass a <Spark>). In a flex column the tiles share
  the card's height, never going below 94px.
-->
<script lang="ts">
  import type { Snippet } from 'svelte'

  export interface LegendItem {
    label: string
    /** CSS colour of the swatch. */
    color: string
    /** A live value shown after the label. */
    value?: string
  }

  const {
    name,
    note,
    noteTitle,
    legend,
    children,
  }: {
    name: string
    /** Small text after the name (a driver version). */
    note?: string
    /** Tooltip of the note. */
    noteTitle?: string
    legend: LegendItem[]
    children: Snippet
  } = $props()
</script>

<div class="mtile">
  <span class="mname">{name}{#if note}<i title={noteTitle}>{note}</i>{/if}</span>
  <span class="mleg">
    {#each legend as item (item.label)}
      <span class="lg"><i style:background={item.color}></i>{item.label}{#if item.value !== undefined}<b>{item.value}</b>{/if}</span>
    {/each}
  </span>
  {@render children()}
</div>

<style>
  .mtile {
    position: relative;
    flex: 1 1 0;
    min-height: 94px;
    background: var(--inset);
    border: 1px solid var(--border-2);
    border-radius: 11px;
    overflow: hidden;
  }
  .mname {
    position: absolute;
    top: 0;
    left: 10px;
    height: 20%;
    display: flex;
    align-items: center;
    z-index: 1;
    font-family: var(--mono);
    font-size: 11.5px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text);
  }
  .mname i {
    margin-left: 8px;
    font-style: normal;
    font-weight: 400;
    text-transform: none;
    letter-spacing: 0;
    font-size: 10.5px;
    color: var(--faint);
  }
  .mleg {
    position: absolute;
    top: 0;
    right: 10px;
    height: 20%;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 10px;
    justify-content: flex-end;
    max-width: calc(100% - 64px);
  }
  .lg {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    font-family: var(--mono);
    font-size: 10px;
    color: var(--muted);
    white-space: nowrap;
  }
  .lg i {
    width: 11px;
    height: 3px;
    border-radius: 2px;
    flex: 0 0 auto;
  }
  .lg b {
    color: var(--text);
    font-weight: 500;
  }
</style>
