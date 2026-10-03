<!--
  The S.M.A.R.T. window of one disk, opened from a disk tile of the Disk I/O card (desktop) — see
  mockups/rktopng-desktop.html. Read-only. The exporter has already judged the disk (state, reasons, the
  colour of every figure); this file lays the answer out. Keyboard: Escape closes, Tab stays inside, and
  closing gives the focus back to the tile that opened it.
-->
<script lang="ts">
  import { untrack } from 'svelte'
  import { dashboard } from '../../lib/state/dashboard.svelte'
  import {
    smartCards,
    smartIdentity,
    smartLogs,
    smartLogsNote,
    smartNotice,
    smartSubtitle,
    smartTable,
    stateLabel,
    stateTone,
    tempChart,
    toneColor,
    unavailableText,
  } from '../../lib/data/smart'
  import Card from '../parts/Card.svelte'
  import SmartCards from './SmartCards.svelte'
  import SmartTable from './SmartTable.svelte'
  import SmartTempChart from './SmartTempChart.svelte'

  const smart = dashboard.smart
  const d = $derived(smart.detail)

  let dialog: HTMLElement | undefined = $state()
  let closeButton: HTMLButtonElement | undefined = $state()

  // While the window is open: the page behind does not scroll, the close button has the focus, and closing returns it.
  $effect(() => {
    if (!smart.device) return
    document.body.style.overflow = 'hidden'
    queueMicrotask(() => untrack(() => closeButton)?.focus())
    return () => {
      document.body.style.overflow = ''
      smart.opener?.focus?.()
    }
  })

  function onKeydown(e: KeyboardEvent): void {
    if (!smart.device) return
    if (e.key === 'Escape') {
      e.preventDefault()
      smart.close()
      return
    }
    if (e.key !== 'Tab' || !dialog) return
    const items = [...dialog.querySelectorAll<HTMLElement>('button:not([disabled]), summary, [tabindex="0"]')].filter((el) => el.offsetParent !== null)
    if (items.length === 0) return
    const first = items[0]
    const last = items[items.length - 1]
    const inside = dialog.contains(document.activeElement)
    if (!inside || (e.shiftKey && document.activeElement === first)) {
      e.preventDefault()
      ;(e.shiftKey ? last : first).focus()
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault()
      first.focus()
    }
  }

  const clock = (ms: number): string => new Date(ms).toLocaleTimeString('en-GB')
</script>

<svelte:window onkeydown={onKeydown} />

{#if smart.device}
  <!-- the backdrop closes the window when it is pressed itself (not when something inside is) -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="back" onmousedown={(e) => e.target === e.currentTarget && smart.close()}>
    <div class="modal" role="dialog" aria-modal="true" aria-labelledby="smart-title" bind:this={dialog}>
      <header class="mhead">
        <div class="mid">
          <h2 id="smart-title"><span class="dev">{smart.device}</span>{#if d?.identity.model}<span class="model">{d.identity.model}</span>{/if}</h2>
          {#if d?.available}<div class="sub">{smartSubtitle(d)}</div>{/if}
        </div>
        {#if d?.available}
          {@const t = stateTone(d.health.state)}
          <span class="badge {t}"><i></i>{stateLabel(d.health.state)}</span>
        {/if}
        <button type="button" class="close" aria-label="Close" bind:this={closeButton} onclick={() => smart.close()}>×</button>
      </header>

      <div class="mbody">
        {#if smart.status === 'failed'}
          <div class="panel" role="alert">
            <p>Could not read the S.M.A.R.T. report ({smart.error}).</p>
            <button type="button" onclick={() => smart.load()}>Try again</button>
          </div>
        {:else if !d}
          <p class="reading" aria-live="polite">Reading S.M.A.R.T. …</p>
        {:else if !d.available}
          <div class="panel">
            <p>{unavailableText(d)}</p>
            <button type="button" onclick={() => smart.load()}>Try again</button>
          </div>
        {:else}
          {@const chart = tempChart(d)}
          {@const notice = smartNotice(d)}
          {@const note = smartLogsNote(d)}
          <div class="stack">
            <div class="verdict {stateTone(d.health.state)}">
              {#if d.health.state === 'ok'}
                {#if d.identity.known_model === false}
                  <b>No problems found in what this drive reports.</b> Its self-assessment passed.
                {:else}
                  <b>No problems found.</b> The drive’s self-assessment passed and no attribute is outside its limits.
                {/if}
              {:else}
                <ul>
                  {#each d.health.reasons ?? [] as r (r.code + r.text)}
                    <li><i style:background={toneColor(r.level === 'fail' ? 'crit' : 'warn')}></i><span>{r.text}</span></li>
                  {/each}
                </ul>
              {/if}
            </div>
            {#if notice}<div class="notice">{notice}</div>{/if}

            <SmartCards cards={smartCards(d)} />
            {#if chart}<SmartTempChart {chart} />{/if}

            <div class="cols">
              <Card title="Identity">
                <dl class="kv">
                  {#each smartIdentity(d) as r (r.label)}
                    <dt>{r.label}</dt>
                    <dd class:dim={r.dim} class:rich={r.parts}>
                      {#if r.parts}
                        {#each r.parts as p, n (n)}
                          {#if p.tip}
                            <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
                            <span class="tip" tabindex="0" data-tip={p.tip}>{p.text}</span>
                          {:else}{p.text}{/if}
                        {/each}
                      {:else}{r.value}{/if}
                      {#if r.sub}<span class="sub">{r.sub}</span>{/if}
                    </dd>
                  {/each}
                </dl>
              </Card>
              <Card title="Error log" small="& self-tests">
                <dl class="kv">
                  {#each smartLogs(d) as r (r.label)}<dt>{r.label}</dt><dd class:dim={r.dim}>{r.value}</dd>{/each}
                </dl>
                {#if note}<p class="note">{note}</p>{/if}
              </Card>
            </div>

            <SmartTable table={smartTable(d)} />
            <p class="foot">Read with smartctl {d.smartctl ?? ''} at {clock(d.read_at)} · cached for up to 30 s</p>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .back {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 4vh 20px;
    background: rgba(3, 6, 9, 0.74);
    backdrop-filter: blur(3px);
  }
  .modal {
    width: 100%;
    max-width: 1000px;
    max-height: 92vh;
    display: flex;
    flex-direction: column;
    background: var(--bg);
    border: 1px solid var(--border-2);
    border-radius: var(--r);
    box-shadow: 0 24px 80px rgba(0, 0, 0, 0.65);
  }
  .mhead {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 15px 20px;
    background: var(--surface);
    border-bottom: 1px solid var(--border);
    border-radius: var(--r) var(--r) 0 0;
  }
  .mid {
    min-width: 0;
  }
  h2 {
    margin: 0;
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: 4px 10px;
    font-size: 17px;
    font-weight: 700;
    letter-spacing: 0.2px;
  }
  .dev {
    font-family: var(--mono);
  }
  .model {
    font-size: 14px;
    font-weight: 500;
    color: var(--muted);
  }
  .sub {
    margin-top: 3px;
    font-family: var(--mono);
    font-size: 11.5px;
    color: var(--muted);
  }
  .badge {
    margin-left: auto;
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 5px 13px;
    border-radius: 20px;
    border: 1px solid;
    font-family: var(--mono);
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.07em;
  }
  .badge i {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: currentColor;
  }
  .badge.ok {
    color: var(--good);
    border-color: rgba(70, 194, 90, 0.4);
    background: rgba(70, 194, 90, 0.08);
  }
  .badge.warn {
    color: var(--warn);
    border-color: rgba(230, 179, 74, 0.45);
    background: rgba(230, 179, 74, 0.09);
  }
  .badge.crit {
    color: var(--crit);
    border-color: rgba(242, 86, 77, 0.5);
    background: rgba(242, 86, 77, 0.1);
  }
  .badge.info {
    color: var(--muted);
    border-color: var(--border-2);
    background: var(--inset);
  }
  .close {
    appearance: none;
    flex: 0 0 auto;
    margin-left: auto;
    width: 30px;
    height: 30px;
    padding: 0;
    border-radius: 50%;
    cursor: pointer;
    font: inherit;
    font-size: 20px;
    line-height: 1;
    border: 1px solid var(--border-2);
    background: var(--inset);
    color: var(--muted);
    display: grid;
    place-items: center;
  }
  .badge + .close {
    margin-left: 0;
  }
  .close:hover {
    color: var(--text);
    border-color: var(--accent);
  }
  .close:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .mbody {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 18px 20px 16px;
  }
  .stack {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .verdict {
    padding: 11px 14px;
    border-radius: var(--r-sm);
    border: 1px solid;
    font-size: 12.5px;
  }
  .verdict.ok {
    border-color: rgba(70, 194, 90, 0.3);
    background: rgba(70, 194, 90, 0.06);
  }
  .verdict.warn {
    border-color: rgba(230, 179, 74, 0.35);
    background: rgba(230, 179, 74, 0.07);
  }
  .verdict.crit {
    border-color: rgba(242, 86, 77, 0.4);
    background: rgba(242, 86, 77, 0.08);
  }
  .verdict ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .verdict li {
    display: flex;
    align-items: baseline;
    gap: 9px;
  }
  .verdict li + li {
    margin-top: 4px;
  }
  .verdict li i {
    flex: 0 0 auto;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    transform: translateY(-1px);
  }
  .notice {
    margin-top: -4px;
    padding: 8px 12px;
    border: 1px dashed var(--border-2);
    border-radius: var(--r-sm);
    font-size: 11.5px;
    color: var(--muted);
  }

  .cols {
    display: grid;
    grid-template-columns: 5fr 7fr;
    gap: 14px;
  }
  @media (max-width: 860px) {
    .cols {
      grid-template-columns: 1fr;
    }
  }
  .kv {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 8px 18px;
    margin: 0;
    font-size: 12.5px;
  }
  .kv dt {
    color: var(--muted);
    white-space: nowrap;
  }
  .kv dd {
    margin: 0;
    text-align: right;
    min-width: 0;
    font-family: var(--mono);
    font-size: 12px;
    color: var(--text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .kv dd.dim {
    color: var(--faint);
  }
  /* a value with explained units: the tooltip must not be clipped by the single-line box */
  .kv dd.rich {
    overflow: visible;
  }
  .tip {
    position: relative;
    cursor: help;
    border-bottom: 1px dotted var(--faint);
  }
  .tip:hover,
  .tip:focus {
    outline: 0;
    border-bottom-color: var(--muted);
  }
  .tip::after {
    content: attr(data-tip);
    position: absolute;
    z-index: 5;
    top: calc(100% + 4px);
    right: 0;
    width: max-content;
    max-width: 270px;
    padding: 8px 10px;
    background: var(--surface);
    border: 1px solid var(--border-2);
    border-radius: var(--r-sm);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
    color: var(--text);
    font: 400 11.5px/1.45 var(--sans);
    text-align: left;
    white-space: normal;
    pointer-events: none;
    opacity: 0;
    visibility: hidden;
    transition: opacity 0.12s;
  }
  .tip:hover::after,
  .tip:focus::after {
    opacity: 1;
    visibility: visible;
  }
  @media (prefers-reduced-motion: reduce) {
    .tip::after {
      transition: none;
    }
  }
  /* a note under the value */
  .kv dd .sub {
    display: block;
    margin-top: 1px;
    font-size: 10.5px;
    color: var(--faint);
    white-space: normal;
  }
  .note {
    margin: 12px 0 0;
    padding-top: 11px;
    border-top: 1px solid var(--border-2);
    font-size: 11.5px;
    color: var(--faint);
  }
  .foot {
    margin: 0;
    text-align: center;
    font-size: 11px;
    color: var(--faint);
  }

  .reading {
    margin: 0;
    padding: 30px 0;
    text-align: center;
    color: var(--muted);
  }
  .panel {
    padding: 22px 16px;
    text-align: center;
    color: var(--muted);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r);
  }
  .panel p {
    margin: 0 0 14px;
    line-height: 1.5;
  }
  .panel button {
    appearance: none;
    padding: 5px 16px;
    border: 1px solid var(--border-2);
    border-radius: 20px;
    background: var(--inset);
    color: var(--accent);
    font: inherit;
    font-size: 12px;
    cursor: pointer;
  }
  .panel button:hover {
    border-color: var(--accent);
  }
</style>
