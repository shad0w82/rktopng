<!-- A temperature tile: name, current value in °C, a stripe in the colour of the level, and the recent history behind the text. -->
<script lang="ts">
  import { drawTemp } from '../../lib/charts/draw'
  import { palette } from '../../lib/charts/theme'
  import { levelColor, tempLevel } from '../../lib/data/thresholds'
  import { dashboard } from '../../lib/state/dashboard.svelte'
  import Spark from './Spark.svelte'
  import Stat from './Stat.svelte'

  const {
    label,
    celsius,
    series,
  }: {
    label: string
    /** undefined: no reading (shown as n/a). */
    celsius: number | undefined
    /** Key of the history of this temperature. */
    series: string
  } = $props()

  const { live } = dashboard
</script>

<Stat
  variant="zone"
  {label}
  value={celsius === undefined ? undefined : Math.round(celsius)}
  unit="°C"
  na={celsius === undefined}
  stripe={celsius === undefined ? 'var(--border-2)' : levelColor(tempLevel(celsius))}
>
  <Spark left="7px" draw={(g, w, h) => drawTemp(g, w, h, live.history.get(series), palette())} />
</Stat>
