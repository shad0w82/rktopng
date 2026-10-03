<script lang="ts">
  import { onMount } from 'svelte'
  import { dashboard } from './lib/state/dashboard.svelte'
  import { ui } from './lib/state/ui.svelte'
  import Desktop from './ui/Desktop.svelte'
  import Mobile from './ui/Mobile.svelte'

  // One connection for the whole app, whichever interface is shown.
  onMount(() => {
    dashboard.start()
    return () => dashboard.stop()
  })
</script>

{#if ui.mode === 'mobile'}
  <Mobile />
{:else}
  <Desktop />
{/if}
