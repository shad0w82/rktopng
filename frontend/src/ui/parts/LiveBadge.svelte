<!-- The connection indicator: a dot and a word. Green and pulsing while data arrives. -->
<script lang="ts">
  import { dashboard } from '../../lib/state/dashboard.svelte'

  const { conn } = dashboard

  const states = {
    live: { color: 'var(--good)', text: 'live', rate: '- 1s' },
    connecting: { color: 'var(--faint)', text: 'connecting…', rate: '' },
    stale: { color: 'var(--warn)', text: 'stale', rate: '' },
    offline: { color: 'var(--crit)', text: 'offline', rate: '' },
  } as const
  const state = $derived(states[conn.status])
</script>

<span class="live" style:--dot={state.color}>
  <span class="dot" class:pulse={conn.status === 'live'}></span>
  {state.text}
  {#if state.rate}<span class="rr">{state.rate}</span>{/if}
</span>

<style>
  .live {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--muted);
    white-space: nowrap;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--dot);
  }
  .dot.pulse {
    animation: pulse 2.4s infinite;
  }
  @keyframes pulse {
    0% {
      box-shadow: 0 0 0 0 rgba(70, 194, 90, 0.45);
    }
    70% {
      box-shadow: 0 0 0 6px rgba(70, 194, 90, 0);
    }
    100% {
      box-shadow: 0 0 0 0 rgba(70, 194, 90, 0);
    }
  }
  .rr {
    color: var(--faint);
    font-family: var(--mono);
    font-size: 10.5px;
  }
</style>
