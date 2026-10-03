<!-- A titled card. On the desktop grid `span` is its width in columns (of 12). -->
<script lang="ts">
  import type { Snippet } from 'svelte'
  import { useLayout } from '../layout'

  const {
    title = '',
    small = '',
    span,
    badge,
    children,
  }: { title?: string; small?: string; span?: number; badge?: Snippet; children: Snippet } = $props()

  // desktop: cards of a row stretch to the same height, so their content is a flex column
  const stretch = useLayout() === 'desktop'
</script>

<div class="card {span ? `s${span}` : ''}" class:stretch>
  {#if title || badge}
    <div class="chead">
      <h3>{title}{#if small}&nbsp;<small>{small}</small>{/if}</h3>
      {@render badge?.()}
    </div>
  {/if}
  {@render children()}
</div>

<style>
  .card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r);
    padding: 14px;
    min-width: 0;
  }
  .card.stretch {
    display: flex;
    flex-direction: column;
  }
  .chead {
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-height: 23px;
    margin-bottom: 12px;
  }
  h3 {
    margin: 0;
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--muted);
    font-weight: 600;
  }
  /* qualifier next to a title, e.g. (R+W) */
  small {
    font-size: inherit;
    font-weight: 400;
    letter-spacing: 0.03em;
    text-transform: none;
    color: var(--faint);
  }
</style>
