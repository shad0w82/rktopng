<!--
  A group of bar rows with a vertical tag on the left (A55, NPU, ZFS…). Groups
  stacked in one container are split by a rule. `one`: a single row, centred on the tag.
  With `more`, a round "+" button under the rows reveals those extra rows (the "×" turns it back).
-->
<script lang="ts">
  import type { Snippet } from 'svelte'

  const {
    label,
    one = false,
    more,
    expandLabel = 'show all',
    collapseLabel = 'hide',
    children,
  }: {
    label: string
    one?: boolean
    /** Extra rows, hidden until the "+" button is pressed. */
    more?: Snippet
    expandLabel?: string
    collapseLabel?: string
    children: Snippet
  } = $props()

  let open = $state(false)
</script>

{#snippet main()}
  <!-- An ordinary horizontal box stretched to the group's height; the vertical text sits in an inner span that
       flex centres. (A vertical writing-mode box that must stretch is fragile: Firefox can leave the text at the top after a reflow.) -->
  <div class="clabel"><span>{label}</span></div>
  <div class="crows" class:one>{@render children()}</div>
{/snippet}

<div class="cgroup" class:stack={!!more}>
  {#if more}
    <div class="cgmain">{@render main()}</div>
    <div class="vexprow">
      <button type="button" class="vexp" class:open aria-expanded={open} aria-label={open ? collapseLabel : expandLabel} onclick={() => (open = !open)}>
        <span class="p">+</span>
      </button>
    </div>
    {#if open}
      <!-- lines up under the rows: label (14) + gap (11) -->
      <div class="cgextra"><div class="crows">{@render more()}</div></div>
    {/if}
  {:else}
    {@render main()}
  {/if}
</div>

<style>
  .cgroup {
    display: flex;
    align-items: stretch;
    gap: 11px;
    padding: 11px 0;
    border-top: 1px solid var(--border-2);
  }
  .cgroup:first-child {
    padding-top: 2px;
    border-top: 0;
  }
  .cgroup:last-child {
    padding-bottom: 0;
  }
  .cgroup.stack {
    display: block;
  }
  .cgmain {
    display: flex;
    align-items: stretch;
    gap: 11px;
  }
  .cgextra {
    padding-left: 25px;
  }
  .clabel {
    flex: 0 0 14px;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .clabel > span {
    writing-mode: vertical-rl;
    transform: rotate(180deg);
    white-space: nowrap;
    line-height: 1;
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.1em;
    color: var(--muted);
    font-weight: 600;
  }
  .crows {
    flex: 1;
    min-width: 0;
  }
  .crows > :global(:first-child) {
    margin-top: 0;
  }
  .crows > :global(:last-child) {
    margin-bottom: 0;
  }
  .crows.one {
    display: flex;
    flex-direction: column;
    justify-content: center;
  }

  .vexprow {
    display: flex;
    justify-content: center;
    padding: 8px 0;
  }
  .vexp {
    flex: 0 0 auto;
    width: 24px;
    height: 24px;
    border-radius: 50%;
    padding: 0;
    cursor: pointer;
    border: 1px solid var(--border-2);
    background: var(--inset);
    color: var(--muted);
    display: grid;
    place-items: center;
  }
  .vexp:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .p {
    font-size: 19px;
    line-height: 1;
    font-weight: 400;
    display: block;
    transition: transform 0.25s ease;
  }
  .vexp.open {
    color: var(--accent);
    border-color: var(--accent);
  }
  .vexp.open .p {
    transform: rotate(135deg); /* the "+" spins into a "×" */
  }
</style>
