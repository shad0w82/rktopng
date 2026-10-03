<!-- Uptime, kernel and OS of the board. The parent sets the arrangement with --gap, --justify and --wrap. -->
<script lang="ts">
  import { fmtUptime } from '../../lib/data/format'
  import { M, val } from '../../lib/data/metrics'
  import { dashboard } from '../../lib/state/dashboard.svelte'

  const { info, live } = dashboard
  const board = $derived(info.board)
  const uptime = $derived(val(live.snapshot, M.uptime))
</script>

<div class="hinfo">
  <div class="it"><span class="k">uptime</span><span class="v">{uptime === undefined ? 'n/a' : fmtUptime(uptime)}</span></div>
  <div class="it"><span class="k">kernel</span><span class="v">{board?.kernel || 'n/a'}</span></div>
  <div class="it"><span class="k">os</span><span class="v">{board?.os || 'n/a'}</span></div>
</div>

<style>
  .hinfo {
    display: flex;
    flex-wrap: var(--wrap, wrap);
    justify-content: var(--justify, space-between);
    gap: var(--gap, 9px 10px);
  }
  .it {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .k {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
  }
  .v {
    font-family: var(--mono);
    font-size: 11.5px;
    color: var(--text);
  }
</style>
