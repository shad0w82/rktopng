<!-- Logo, name and board of the dashboard. -->
<script lang="ts">
  import { dashboard } from '../../lib/state/dashboard.svelte'

  const board = $derived(dashboard.info.board)
</script>

<div class="brand">
  <img class="logo" src="favicon.png" alt="" width="34" height="34" />
  <div class="nm">
    <b>RkTopNG</b>
    <div class="sub">
      {#if board}<span class="board">{board.model}</span> · {board.vendor} {board.soc}{:else}—{/if}
    </div>
  </div>
</div>

<style>
  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
  }
  .logo {
    width: 34px;
    height: 34px;
    object-fit: contain;
    flex: 0 0 auto;
  }
  .nm {
    min-width: 0;
  }
  b {
    display: block;
    font-size: 17px;
    font-weight: 700;
    letter-spacing: 0.2px;
  }
  .sub {
    font-size: 11px;
    color: var(--muted);
    line-height: 1.35;
  }
  .board {
    color: var(--text);
  }
</style>
