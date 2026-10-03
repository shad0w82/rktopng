<!--
  One row of a bar list: name | bar | "NN% | trailing value" (a frequency, a size…).
  The numbers are aligned in columns (tabular figures, fixed widths) so rows line up.
-->
<script lang="ts">
  const {
    name,
    percent,
    color,
    trail = '',
    nameCol = 'auto',
    trailCh = 8,
    pctWidth = 'calc(4ch + 2px)',
    text,
    minFill = 0,
  }: {
    name: string
    /** 0–100; undefined shows an empty bar and a dash. */
    percent: number | undefined
    /** CSS colour of the bar. */
    color: string
    /** Text after the separator. */
    trail?: string
    /** Width of the name column: `auto` hugs the text, a fixed width lines bars up across groups. */
    nameCol?: string
    /** Width of the trailing column, in characters. */
    trailCh?: number
    /** Width of the "NN%" cell: number, 2px, percent sign. */
    pctWidth?: string
    /** What to print instead of the rounded percentage (e.g. "<1"). */
    text?: string
    /** Smallest bar, in %, so that a tiny but non-zero value stays visible. */
    minFill?: number
  } = $props()

  const width = $derived(percent === undefined ? 0 : Math.max(minFill, Math.min(100, percent)))
</script>

<div class="row" style:--name-col={nameCol}>
  <span class="n"><b>{name}</b></span>
  <div class="track"><div class="fill" style:width="{width}%" style:background={color}></div></div>
  <span class="val">
    <span class="mp" style:width={pctWidth}><b>{text ?? (percent === undefined ? '–' : Math.round(percent))}</b>%</span><span class="vsep">|</span><span
      class="trail"
      style:width="{trailCh}ch">{trail}</span
    >
  </span>
</div>

<style>
  .row {
    display: grid;
    grid-template-columns: var(--name-col) 1fr auto;
    align-items: center;
    gap: 9px;
    margin: 7px 0;
  }
  .n {
    font-family: var(--mono);
    font-size: 11.5px;
    color: var(--muted);
  }
  .n b {
    color: var(--text);
  }
  .track {
    height: 7px;
    border-radius: 6px;
    background: #33475b;
    overflow: hidden;
  }
  .fill {
    height: 100%;
    border-radius: 6px;
    transition:
      width 0.5s ease,
      background-color 0.4s ease;
  }
  .val {
    display: inline-flex;
    align-items: baseline;
    justify-content: flex-end;
    font-family: var(--mono);
    font-size: 11.5px;
    font-variant-numeric: tabular-nums;
    color: var(--muted);
  }
  .mp {
    display: inline-block;
    text-align: right;
  }
  .mp b {
    color: var(--text);
    margin-right: 2px;
  }
  .vsep {
    color: var(--faint);
    margin: 0 6px;
  }
  .trail {
    display: inline-block;
    text-align: right;
    color: var(--faint);
  }
</style>
