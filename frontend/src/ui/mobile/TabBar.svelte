<!-- The row of section buttons under the gauges: icons only, all visible (no swiping). -->
<script lang="ts">
  import { ICONS, type IconName } from '../parts/icons'

  export interface Tab {
    key: string
    /** Spoken name and tooltip. */
    label: string
    icon: IconName
  }

  let { tabs, current = $bindable() }: { tabs: Tab[]; current: string } = $props()
</script>

<div class="tabs" role="tablist" aria-label="Sections">
  {#each tabs as t (t.key)}
    <button
      type="button"
      role="tab"
      class="tab"
      class:on={current === t.key}
      aria-selected={current === t.key}
      aria-label={t.label}
      title={t.label}
      onclick={() => (current = t.key)}
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        {@html ICONS[t.icon]}
      </svg>
    </button>
  {/each}
</div>

<style>
  .tabs {
    display: flex;
    gap: 6px;
    padding: 13px 0 8px;
  }
  .tab {
    flex: 1;
    min-width: 0;
    display: grid;
    place-items: center;
    padding: 9px 0;
    border-radius: 11px;
    border: 1px solid var(--border-2);
    background: var(--inset);
    color: var(--muted);
    cursor: pointer;
  }
  .tab svg {
    width: 20px;
    height: 20px;
  }
  .tab.on {
    background: var(--accent);
    border-color: var(--accent);
    color: #052421;
  }
  .tab:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
</style>
