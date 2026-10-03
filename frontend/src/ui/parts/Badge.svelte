<!-- A small pill next to a card title. Put a <b> inside for the brighter value. -->
<script lang="ts">
  import type { Snippet } from 'svelte'

  const { children }: { children: Snippet } = $props()
</script>

<span class="badge">{@render children()}</span>

<style>
  .badge {
    font-family: var(--mono);
    font-size: 11px;
    color: var(--muted);
    background: var(--inset);
    border: 1px solid var(--border-2);
    border-radius: 20px;
    padding: 3px 10px;
  }
  .badge :global(b) {
    color: var(--text);
    font-weight: 500;
  }
</style>
