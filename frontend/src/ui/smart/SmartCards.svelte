<!-- The eight headline figures of the S.M.A.R.T. window. A figure the drive does not report reads "n/a". -->
<script lang="ts">
  import { toneColor, type SmartCard } from '../../lib/data/smart'

  const { cards }: { cards: SmartCard[] } = $props()
</script>

<div class="grid">
  {#each cards as c (c.label)}
    <div class="stat" class:na={c.value === null} title={c.source ? `Source: ${c.source}` : undefined}>
      <div class="stripe" style:background={c.value === null ? undefined : toneColor(c.tone)}></div>
      <div class="l">{c.label}</div>
      <div class="big">
        {#if c.value === null}n/a{:else}{c.value}{#if c.unit}<span class="u" class:word={c.unit.length > 1}>{c.unit}</span>{/if}{/if}
      </div>
      <div class="s">{c.sub}</div>
    </div>
  {/each}
</div>

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 10px;
  }
  @media (max-width: 860px) {
    .grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }
  .stat {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r);
    padding: 11px 13px;
    position: relative;
    overflow: hidden;
  }
  .stripe {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    width: 3px;
    background: var(--border-2);
  }
  .l {
    font-size: 10.5px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
  }
  .big {
    font-family: var(--mono);
    font-size: 24px;
    font-weight: 500;
    margin-top: 2px;
    letter-spacing: -0.5px;
  }
  .u {
    font-size: 13px;
    color: var(--muted);
    margin-left: 2px;
  }
  .u.word {
    margin-left: 5px; /* TB, GB: a word, not a symbol */
  }
  .s {
    margin-top: 1px;
    font-size: 10.5px;
    line-height: 1.35;
    color: var(--faint);
    font-family: var(--mono);
  }
  .na .big {
    color: var(--faint);
    font-size: 20px;
    padding-top: 4px;
  }
</style>
