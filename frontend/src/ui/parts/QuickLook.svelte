<!-- The gauges (SoC temperature, CPU, RAM) and the fan: what is worth a glance at any time. -->
<script lang="ts">
  import { drawFan } from '../../lib/charts/draw'
  import { palette } from '../../lib/charts/theme'
  import { cpuAverage, fanLevel, ramUsedPct } from '../../lib/data/derive'
  import { K } from '../../lib/data/history'
  import { M, val } from '../../lib/data/metrics'
  import { cpuLevel, levelColor, ramLevel, tempLevel } from '../../lib/data/thresholds'
  import { dashboard } from '../../lib/state/dashboard.svelte'
  import Gauge from './Gauge.svelte'
  import Spark from './Spark.svelte'
  import Stat from './Stat.svelte'

  /** embedded: inside a card (desktop); bare: straight on the page, just under the header (phone). */
  const { variant = 'embedded' }: { variant?: 'embedded' | 'bare' } = $props()

  const { live } = dashboard

  const snap = $derived(live.snapshot)
  const soc = $derived(val(snap, M.temp, { zone: 'soc' }))
  const cpu = $derived(cpuAverage(snap))
  const ram = $derived(ramUsedPct(snap))
  const fan = $derived(fanLevel(snap))

  const round = (v: number | undefined) => (v === undefined ? undefined : Math.round(v))
</script>

<div class={['quick', variant]}>
  <div class="gauges">
    <Gauge name="SOC" unit="°C" value={round(soc)} fraction={(soc ?? 0) / 100} color={levelColor(tempLevel(soc ?? 0))} />
    <Gauge name="CPU" unit="%" value={round(cpu)} fraction={(cpu ?? 0) / 100} color={levelColor(cpuLevel(cpu ?? 0))} />
    <Gauge name="RAM" unit="%" value={round(ram)} fraction={(ram ?? 0) / 100} color={levelColor(ramLevel(ram ?? 0))} />
  </div>
  <div class="fan">
    <Stat variant="fan" label="fan" value={round(fan?.pct)} unit="%">
      {#snippet sub()}<span>{fan ? `${Math.round(fan.raw)} / 255` : '–'}</span>{/snippet}
      <Spark draw={(g, w, h) => drawFan(g, w, h, live.history.get(K.fanPct), palette())} />
    </Stat>
  </div>
</div>

<style>
  .gauges {
    display: flex;
    gap: 7px;
    padding: 6px 0 2px;
  }
  .gauges :global(.gauge) {
    flex: 1;
    min-width: 0;
  }
  .fan {
    margin: 14px 6px 2px;
  }
  /* on the phone the gauges sit on the page itself, a little more spaced */
  .bare {
    padding: 0 6px;
  }
  .bare .gauges {
    padding: 15px 0 2px;
  }
  .bare .fan {
    margin: 14px 8px 2px;
  }
</style>
