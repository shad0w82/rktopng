<!-- A grid of equal tiles: `cols` columns, `mdCols` (2) up to 1100px, `smCols` (1) up to 760px. A short last row leaves its cells empty. -->
<script lang="ts">
  import type { Snippet } from 'svelte'
  import { useLayout } from '../layout'

  const { cols = 4, mdCols = 2, smCols = 1, children }: { cols?: number; mdCols?: number; smCols?: number; children: Snippet } = $props()

  // the phone interface is as narrow as the narrowest window, whatever the screen it is shown on
  const phone = useLayout() === 'mobile'
</script>

<div class="tiles" class:phone style:--cols={cols} style:--md-cols={mdCols} style:--sm-cols={smCols}>{@render children()}</div>

<style>
  .tiles {
    display: grid;
    grid-template-columns: repeat(var(--cols), 1fr);
    gap: 10px;
  }
  @media (max-width: 1100px) {
    .tiles {
      grid-template-columns: repeat(var(--md-cols), 1fr);
    }
  }
  @media (max-width: 760px) {
    .tiles {
      grid-template-columns: repeat(var(--sm-cols), 1fr);
    }
  }
  .tiles.phone {
    grid-template-columns: repeat(var(--sm-cols), 1fr);
  }
</style>
