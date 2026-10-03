<!-- The attribute table (SATA) or health log (NVMe), a "Critical only" filter, tooltips on the column headers, and the rules of the state. -->
<script lang="ts">
  import { toneColor, type SmartTable } from '../../lib/data/smart'
  import Card from '../parts/Card.svelte'

  const { table }: { table: SmartTable } = $props()

  let filter = $state<'all' | 'crit'>('all')
  const rows = $derived(filter === 'crit' ? table.rows.filter((r) => r.critical) : table.rows)
  const ata = $derived(table.kind === 'ata')
</script>

<Card title={table.title}>
  {#snippet badge()}
    <div class="seg" role="group" aria-label="Rows shown">
      <button type="button" class:on={filter === 'all'} onclick={() => (filter = 'all')}>All</button>
      <button type="button" class:on={filter === 'crit'} onclick={() => (filter = 'crit')}>Critical only</button>
    </div>
  {/snippet}
  <table>
    <thead>
      <tr>
        <th></th>
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        {#if ata}<th tabindex="0" data-tip="Attribute number (decimal · hex), defined by the drive’s vendor."><span>ID</span></th>{/if}
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <th tabindex="0" data-tip={ata ? 'Name from smartctl’s drive database, which knows each vendor’s real names.' : 'Field of the drive’s NVMe health log.'}><span>Attribute</span></th>
        {#if ata}
          <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
          <th class="r" tabindex="0" data-tip="Score the drive computes from the raw value. Higher is better and it falls as things get worse. The scale is vendor-specific (often 100 = new)."><span>Normalized</span></th>
          <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
          <th class="r" tabindex="0" data-tip="Lowest Normalized value ever recorded. It never goes back up, so it reveals past problems."><span>Worst</span></th>
          <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
          <th class="r" tabindex="0" data-tip="If Normalized falls to this value or below, the drive reports the attribute as failed. — means no threshold: it can never fail."><span>Thresh</span></th>
          <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
          <th class="r" tabindex="0" data-tip="The actual count or measurement, in the attribute’s own unit (hours, sectors, °C…)."><span>Raw</span></th>
        {:else}
          <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
          <th class="r" tabindex="0" data-tip="Current value reported by the drive."><span>Value</span></th>
          <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
          <th class="r" tabindex="0" data-tip="Limit the drive sets for this field; the state changes when the value crosses it. — means none."><span>Threshold</span></th>
        {/if}
      </tr>
    </thead>
    <tbody>
      {#each rows as r (r.key)}
        <tr>
          <td class="st"><i class="dot" style:background={toneColor(r.tone)}></i></td>
          {#if ata}<td class="id">{r.id}</td>{/if}
          <td class="nm">{r.name}{#if r.critical}<span class="tag">critical</span>{/if}{#if r.note}<small>{r.note}</small>{/if}</td>
          {#if ata}
            <td class="r num">{r.normalized}</td>
            <td class="r num">{r.worst}</td>
            <td class="r num">{r.thresh}</td>
            <td class="r num">{r.value}{#if r.hint}<span class="hu">{r.hint}</span>{/if}</td>
          {:else}
            <td class="r num">{r.value}{#if r.hint}<span class="hu">{r.hint}</span>{/if}</td>
            <td class="r num">{r.thresh}</td>
          {/if}
        </tr>
      {/each}
    </tbody>
  </table>
</Card>

<details class="rules">
  <summary>How the state is decided</summary>
  <dl>
    <dt>Any drive</dt>
    <dd>
      <b>Failing</b> — the drive’s own self-assessment did not pass.<br />
      <b>Warning</b> — the temperature is at or above the drive’s limit.
    </dd>
    <dt>NVMe</dt>
    <dd>
      <b>Failing</b> — any critical-warning flag is set · available spare is at or below its threshold · wear ≥ 100%.<br />
      <b>Warning</b> — media errors above 0 · available spare under 50% · wear ≥ 80%.
    </dd>
    <dt>SATA</dt>
    <dd>
      <b>Failing</b> — an attribute is at or below its threshold now (smartctl’s own verdict) · wear ≥ 100%.<br />
      <b>Warning</b> — an attribute reached its threshold in the past · more than 0 reallocated, pending, offline-uncorrectable or uncorrectable sectors, or CRC errors · entries in the ATA error log · wear ≥ 80%.
    </dd>
    <dt>Never counted</dt>
    <dd>Unsafe shutdowns, NVMe error-log entries, the all-time peak temperature and any “worst” value: they are shown, but they do not change the state.</dd>
    <dt>Any brand</dt>
    <dd>
      Figures come from the standard logs first (NVMe health log, ATA device statistics, SCT); vendor attributes only where their meaning is recognised. A figure the drive does not report shows <b>n/a</b>, never a guess.
    </dd>
  </dl>
</details>

<style>
  .seg {
    display: inline-flex;
    border: 1px solid var(--border-2);
    border-radius: 20px;
    overflow: hidden;
  }
  .seg button {
    appearance: none;
    margin: 0;
    border: 0;
    background: var(--inset);
    color: var(--muted);
    font: inherit;
    font-size: 11px;
    padding: 3px 12px;
    cursor: pointer;
  }
  .seg button + button {
    border-left: 1px solid var(--border-2);
  }
  .seg button.on {
    background: rgba(52, 208, 189, 0.14);
    color: var(--accent);
  }
  .seg button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-family: var(--mono);
    font-size: 12px;
  }
  th {
    text-align: left;
    font-weight: 500;
    font-size: 10.5px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--faint);
    padding: 0 10px 9px;
    white-space: nowrap;
  }
  th.r,
  td.r {
    text-align: right;
  }
  td {
    padding: 8px 10px;
    border-top: 1px solid var(--border);
    vertical-align: top;
  }
  td.st {
    width: 28px;
    padding-right: 0;
  }
  td.id,
  td.num {
    color: var(--muted);
    white-space: nowrap;
  }
  td.id {
    color: var(--faint);
  }
  .hu {
    display: block;
    margin-top: 1px;
    font-size: 10.5px;
    color: var(--faint);
  }
  td.nm {
    font-family: var(--sans);
    font-size: 13px;
    color: var(--text);
  }
  td.nm small {
    display: block;
    margin-top: 2px;
    font-size: 11px;
    line-height: 1.35;
    color: var(--faint);
  }
  tbody tr:hover td {
    background: var(--inset);
  }
  .dot {
    display: inline-block;
    width: 9px;
    height: 9px;
    margin-top: 4px;
    border-radius: 50%;
  }
  .tag {
    margin-left: 8px;
    padding: 1px 6px;
    border: 1px solid rgba(52, 208, 189, 0.4);
    border-radius: 10px;
    vertical-align: middle;
    white-space: nowrap;
    font-family: var(--mono);
    font-size: 9.5px;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--accent);
  }

  /* column help: a tooltip on hover or keyboard focus; right-aligned columns open towards the left so they stay inside the window */
  th[data-tip] {
    position: relative;
    cursor: help;
  }
  th[data-tip] > span {
    border-bottom: 1px dotted var(--faint);
  }
  th[data-tip]:hover,
  th[data-tip]:focus {
    color: var(--muted);
    outline: 0;
  }
  th[data-tip]::after {
    content: attr(data-tip);
    position: absolute;
    z-index: 5;
    top: calc(100% + 2px);
    left: 0;
    width: max-content;
    max-width: 270px;
    padding: 8px 10px;
    background: var(--surface);
    border: 1px solid var(--border-2);
    border-radius: var(--r-sm);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
    color: var(--text);
    font: 400 11.5px/1.45 var(--sans);
    letter-spacing: 0;
    text-transform: none;
    white-space: normal;
    text-align: left;
    pointer-events: none;
    opacity: 0;
    visibility: hidden;
    transition: opacity 0.12s;
  }
  th.r[data-tip]::after {
    left: auto;
    right: 0;
  }
  th[data-tip]:hover::after,
  th[data-tip]:focus::after {
    opacity: 1;
    visibility: visible;
  }
  @media (prefers-reduced-motion: reduce) {
    th[data-tip]::after {
      transition: none;
    }
  }

  .rules {
    font-size: 12px;
    color: var(--muted);
  }
  .rules summary {
    cursor: pointer;
    width: max-content;
  }
  .rules summary:hover {
    color: var(--text);
  }
  .rules dl {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 9px 18px;
    margin: 10px 0 0;
    padding: 12px 14px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r-sm);
  }
  .rules dt {
    color: var(--text);
    font-weight: 600;
  }
  .rules dd {
    margin: 0;
    line-height: 1.5;
  }
  .rules b {
    color: var(--text);
    font-weight: 500;
  }
</style>
