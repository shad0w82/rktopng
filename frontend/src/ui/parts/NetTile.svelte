<!--
  One network interface: its name and kind, the download / upload history, and the two
  current rates in a shared unit (▼ download, ▲ upload).
-->
<script lang="ts">
  import { drawNet } from '../../lib/charts/draw'
  import { palette } from '../../lib/charts/theme'
  import { fmtRatePair } from '../../lib/data/format'
  import { M, seriesKey } from '../../lib/data/metrics'
  import { netTag, type NetInterface } from '../../lib/data/network'
  import { dashboard } from '../../lib/state/dashboard.svelte'
  import { useLayout } from '../layout'
  import Spark from './Spark.svelte'

  const { net }: { net: NetInterface } = $props()

  const { live } = dashboard
  const phone = useLayout() === 'mobile'
  const rates = $derived(fmtRatePair(net.rx ?? 0, net.tx ?? 0))
  const tag = $derived(netTag(net))
</script>

<div class="netcard">
  <div class="ifn">{net.iface}<span class="ift">{tag}</span></div>
  <div class="chart" class:phone>
    <Spark
      draw={(g, w, h) =>
        drawNet(
          g,
          w,
          h,
          live.history.get(seriesKey(M.netRx, { iface: net.iface })),
          live.history.get(seriesKey(M.netTx, { iface: net.iface })),
          palette(),
        )}
    />
  </div>
  <div class="rt">
    <span class="dn">▼ <b>{net.rx === undefined ? '–' : rates.a}</b></span>
    <span class="up">▲ <b>{net.tx === undefined ? '–' : rates.b}</b></span>
    <span class="un">{rates.unit}</span>
  </div>
</div>

<style>
  .netcard {
    background: var(--inset);
    border: 1px solid var(--border);
    border-radius: var(--r-sm);
    padding: 11px 12px;
    min-width: 0;
  }
  .ifn {
    font-family: var(--mono);
    font-size: 12px;
    color: var(--text);
    display: flex;
    justify-content: space-between;
  }
  .ift {
    color: var(--faint);
  }
  .chart {
    position: relative;
    height: 64px;
    margin: 7px 0;
  }
  .chart.phone {
    height: 48px;
  }
  .rt {
    display: flex;
    align-items: baseline;
    gap: 12px;
    font-family: var(--mono);
    font-size: 12px;
  }
  .dn {
    color: var(--info);
  }
  .up {
    color: var(--accent);
  }
  .dn b,
  .up b {
    font-weight: 600;
  }
  .un {
    color: var(--faint);
    margin-left: auto;
  }
</style>
