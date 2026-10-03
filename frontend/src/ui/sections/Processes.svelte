<!--
  Processes: a btop-like table. Click a column header to sort by it, again to flip the order
  (the server returns the other end of the whole list, not the same rows upside down). The
  list is fetched every 2 s, but only while the card is on screen and the tab is in front.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import type { ProcessSort } from '../../lib/api/types'
  import { fmtProcMem } from '../../lib/data/format'
  import { commandOf, isDescending } from '../../lib/data/processes'
  import { dashboard } from '../../lib/state/dashboard.svelte'
  import { useLayout } from '../layout'
  import Badge from '../parts/Badge.svelte'
  import Card from '../parts/Card.svelte'
  import SectionHead from '../parts/SectionHead.svelte'

  const { procs } = dashboard
  const phone = useLayout() === 'mobile'
  procs.limit = phone ? 10 : 15

  // the phone's sort buttons (a tap on the active one flips the order)
  const pills: Array<{ key: ProcessSort; label: string }> = [
    { key: 'cpu', label: 'CPU%' },
    { key: 'mem', label: 'Mem' },
    { key: 'name', label: 'Name' },
    { key: 'pid', label: 'PID' },
    { key: 'user', label: 'User' },
    { key: 'threads', label: 'Thr' },
  ]

  const columns: Array<{ label: string; key?: ProcessSort; right?: boolean; cls?: string }> = [
    { label: 'PID', key: 'pid' },
    { label: 'Program', key: 'name' },
    { label: 'Command', cls: 'cmd' },
    { label: 'User', key: 'user' },
    { label: 'Thr', key: 'threads', right: true },
    { label: 'Mem', key: 'mem', right: true },
    { label: 'CPU%', key: 'cpu', right: true },
  ]

  let box = $state<HTMLElement>()
  let onScreen = false

  onMount(() => {
    const sync = () => (onScreen && document.visibilityState === 'visible' ? procs.start() : procs.stop())
    const io = new IntersectionObserver(
      ([entry]) => {
        onScreen = entry.isIntersecting
        sync()
      },
      { rootMargin: '150px' },
    )
    io.observe(box!)
    document.addEventListener('visibilitychange', sync)
    return () => {
      io.disconnect()
      document.removeEventListener('visibilitychange', sync)
      procs.stop()
    }
  })

  const ariaSort = (key: ProcessSort | undefined) =>
    key !== procs.sort ? undefined : isDescending(procs.sort, procs.reverse) ? 'descending' : 'ascending'
</script>

<SectionHead label="Processes" icon="processes" />

<Card title="Processes" span={12}>
  {#snippet badge()}<Badge>{procs.list ? `${procs.list.total} task` : '– task'}</Badge>{/snippet}
{#if phone}
  <div bind:this={box}>
    <div class="sortbar">
      {#each pills as pill (pill.key)}
        <button type="button" class="pill" class:on={procs.sort === pill.key} aria-pressed={procs.sort === pill.key} onclick={() => procs.setSort(pill.key)}>
          {pill.label}{procs.sort === pill.key ? (isDescending(procs.sort, procs.reverse) ? ' ▾' : ' ▴') : ''}
        </button>
      {/each}
    </div>
    <div>
      {#each procs.rows as p (p.pid)}
        <div class="proc">
          <div class="meta">
            <div class="pn">{p.name} <span class="pid">{p.pid} <span class="sep">|</span> {p.user}</span></div>
            <div class="cmd">{commandOf(p)}</div>
          </div>
          <div class="nums">
            <div class="cpu" class:hot={p.cpu >= 5}>{p.cpu.toFixed(1)}%</div>
            <div class="mem">{p.threads} <span class="sep">|</span> {fmtProcMem(p.mem_bytes)}</div>
          </div>
        </div>
      {:else}
        <p class="none">{procs.error ? 'Process list unavailable.' : ''}</p>
      {/each}
    </div>
  </div>
{:else}
  <div class="scroll" bind:this={box}>
    <table class="ptable">
      <colgroup>
        <col class="c-pid" /><col class="c-prog" /><col class="c-cmd" /><col class="c-user" /><col class="c-thr" /><col class="c-mem" /><col class="c-cpu" />
      </colgroup>
      <thead>
        <tr>
          {#each columns as c (c.label)}
            <th class={[c.right && 'r', c.cls, c.key === procs.sort && 'on']} aria-sort={ariaSort(c.key)}>
              {#if c.key}
                {@const key = c.key}
                <button type="button" onclick={() => procs.setSort(key)}>
                  {c.label}<span class="ar">{key === procs.sort ? (isDescending(procs.sort, procs.reverse) ? '▾' : '▴') : ''}</span>
                </button>
              {:else}
                {c.label}
              {/if}
            </th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each procs.rows as p (p.pid)}
          <tr>
            <td class="pid">{p.pid}</td>
            <td class="prog">{p.name}</td>
            <td class="cmd" title={p.cmd}>{commandOf(p)}</td>
            <td class="user">{p.user}</td>
            <td class="r num">{p.threads}</td>
            <td class="r num">{fmtProcMem(p.mem_bytes)}</td>
            <td class="r"><b class:hot={p.cpu >= 5}>{p.cpu.toFixed(1)}</b></td>
          </tr>
        {:else}
          <tr><td class="empty" colspan="7">{procs.error ? 'Process list unavailable.' : ''}</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}
</Card>

<style>
  /* phone: sort buttons and one card per process */
  .sortbar {
    display: flex;
    gap: 6px;
    overflow-x: auto;
    padding-bottom: 8px;
    margin: -2px -2px 6px;
    -webkit-overflow-scrolling: touch;
  }
  .pill {
    flex: 0 0 auto;
    font-size: 11.5px;
    padding: 5px 11px;
    border-radius: 20px;
    border: 1px solid var(--border-2);
    background: var(--inset);
    color: var(--muted);
    white-space: nowrap;
    line-height: 13px; /* the mockup's buttons have the browser's default line height */
    cursor: pointer;
  }
  .pill.on {
    background: rgba(52, 208, 189, 0.14);
    border-color: var(--accent);
    color: var(--accent);
  }
  .pill:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .proc {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 9px 0;
    border-top: 1px solid var(--border);
  }
  .proc:first-child {
    border-top: 0;
  }
  .meta {
    min-width: 0;
    flex: 1;
  }
  .pn {
    font-family: var(--mono);
    font-size: 13px;
    font-weight: 600;
    display: flex;
    gap: 7px;
    align-items: baseline;
  }
  .pn .pid {
    color: var(--faint);
    font-weight: 400;
    font-size: 11px;
  }
  .proc .cmd {
    font-family: var(--mono);
    font-size: 11px;
    color: var(--faint);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .nums {
    text-align: right;
    font-family: var(--mono);
    flex: 0 0 auto;
  }
  .nums .cpu {
    font-size: 14px;
    font-weight: 600;
  }
  .nums .cpu.hot {
    color: var(--warn);
  }
  .nums .mem {
    font-size: 10.5px;
    color: var(--faint);
  }
  .sep {
    opacity: 0.5;
  }
  .none {
    margin: 0;
    color: var(--faint);
    text-align: center;
    font-size: 12px;
  }

  /* on a narrow window the fixed-width columns scroll inside the card instead of pushing the page wider */
  .scroll {
    overflow-x: auto;
  }
  .ptable {
    width: 100%;
    border-collapse: collapse;
    table-layout: fixed;
    font-family: var(--mono);
    font-size: 12.5px;
    line-height: normal; /* as in the mockup, which has no doctype: tables there do not inherit the page's line height */
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
    user-select: none;
  }
  th.r {
    text-align: right;
  }
  th button {
    background: none;
    border: 0;
    padding: 0;
    margin: 0;
    font: inherit;
    letter-spacing: inherit;
    text-transform: inherit;
    color: inherit;
    cursor: pointer;
  }
  th button:hover {
    color: var(--muted);
  }
  th button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 3px;
  }
  th.on,
  th.on button:hover {
    color: var(--accent);
  }
  .ar {
    display: inline-block;
    width: 1em;
    margin-left: 3px;
  }
  td {
    padding: 7px 10px;
    border-top: 1px solid var(--border);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  td.r {
    text-align: right;
  }
  td.pid,
  td.cmd {
    color: var(--faint);
  }
  td.prog {
    color: var(--text);
    font-weight: 600;
  }
  td.user,
  td.num {
    color: var(--muted);
  }
  td b {
    font-weight: 600;
    color: var(--text);
  }
  td b.hot {
    color: var(--warn);
  }
  td.empty {
    color: var(--faint);
    text-align: center;
  }
  tbody tr:hover td {
    background: var(--inset);
  }
  .c-pid {
    width: 76px;
  }
  .c-prog {
    width: 180px;
  }
  .c-user {
    width: 96px;
  }
  .c-thr {
    width: 60px;
  }
  .c-mem {
    width: 84px;
  }
  .c-cpu {
    width: 78px;
  }
  @media (max-width: 760px) {
    .c-prog {
      width: 130px;
    }
    .c-cmd,
    th.cmd,
    td.cmd {
      display: none;
    }
  }
</style>
