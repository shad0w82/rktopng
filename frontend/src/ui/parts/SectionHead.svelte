<!--
  Desktop section separator: the section's icon and a rule to the right edge. No visible text
  by design; the name is only in title / aria-label. On the phone the tabs name the sections,
  so nothing is drawn.
-->
<script lang="ts">
  import { useLayout } from '../layout'
  import { ICONS, type IconName } from './icons'

  const { label, icon }: { label: string; icon: IconName } = $props()
  const layout = useLayout()
</script>

{#if layout === 'desktop'}
  <div class="sechead" role="separator" aria-label={label} title={label}>
    <svg viewBox="0 0 24 24" aria-hidden="true">{@html ICONS[icon]}</svg>
  </div>
{/if}

<style>
  .sechead {
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    gap: 10px;
    margin-top: 16px;
  }
  .sechead:first-child {
    margin-top: 0;
  }
  svg {
    width: 18px;
    height: 18px;
    flex: 0 0 auto;
    color: var(--accent);
    fill: none;
    stroke: currentColor;
    stroke-width: 1.7;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .sechead::after {
    content: '';
    flex: 1;
    height: 1px;
    background: var(--border-2);
  }
</style>
