<!-- The drive's own temperature log (ATA SCT): one sample every 10 minutes for the last ~21 hours, kept by the drive itself. -->
<script lang="ts">
  import { CHART_H, CHART_W, type TempChart } from '../../lib/data/smart'
  import Badge from '../parts/Badge.svelte'
  import Card from '../parts/Card.svelte'

  const { chart }: { chart: TempChart } = $props()
</script>

<Card title="Temperature" small="(last {chart.span}, from the drive’s own log)">
  {#snippet badge()}<Badge>now <b>{chart.now} °C</b> · low {chart.min} · high {chart.max}</Badge>{/snippet}
  <div class="wrap">
    <svg viewBox="0 0 {CHART_W} {CHART_H}" preserveAspectRatio="none" role="img" aria-label="Temperature over the last {chart.span}">
      <polygon points={chart.area} fill="rgba(52,208,189,.13)" />
      <polyline points={chart.line} fill="none" stroke="var(--accent)" stroke-width="1.6" vector-effect="non-scaling-stroke" stroke-linejoin="round" />
    </svg>
    <span class="y top">{chart.max} °C</span>
    <span class="y bottom">{chart.min} °C</span>
  </div>
  <div class="axis"><span>{chart.span} ago</span><span>{chart.intervalMinutes} min per sample</span><span>now</span></div>
</Card>

<style>
  .wrap {
    position: relative;
  }
  svg {
    width: 100%;
    height: 96px;
    display: block;
    background: var(--inset);
    border: 1px solid var(--border-2);
    border-radius: 11px;
  }
  .y {
    position: absolute;
    right: 9px;
    font-family: var(--mono);
    font-size: 10px;
    font-weight: 600;
    color: var(--muted);
  }
  .top {
    top: 6px;
  }
  .bottom {
    bottom: 6px;
  }
  .axis {
    display: flex;
    justify-content: space-between;
    margin-top: 7px;
    font-family: var(--mono);
    font-size: 10.5px;
    color: var(--faint);
  }
</style>
