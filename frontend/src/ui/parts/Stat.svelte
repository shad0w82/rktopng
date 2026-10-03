<!--
  A tile: small label, big value, optional sub line, optional colour stripe on the
  left and an optional chart behind the text (pass a <Spark> as the child).
  variant "disk" is the Overview disk-temperature tile, "fan" the tall fan tile, "zone" a wide
  temperature tile with its history behind the text (thermal zones, disk temperature).
-->
<script lang="ts">
  import type { Snippet } from 'svelte'
  import { useLayout } from '../layout'

  const {
    label,
    value,
    unit = '',
    stripe,
    na = false,
    variant = 'plain',
    sub,
    children,
  }: {
    label: string
    value?: string | number
    unit?: string
    /** CSS colour of the left stripe. */
    stripe?: string
    /** No reading: show "n/a" in the faint colour. */
    na?: boolean
    variant?: 'plain' | 'disk' | 'fan' | 'zone'
    sub?: Snippet
    children?: Snippet
  } = $props()

  const compact = useLayout() === 'mobile'
</script>

<div class="stat {variant}" class:na class:compact>
  {#if stripe}<div class="stripe" style:background={stripe}></div>{/if}
  <div class="l">{label}</div>
  <div class="big">{#if na || value === undefined}n/a{:else}<span>{value}</span><span class="u">{unit}</span>{/if}</div>
  {#if sub}<div class="s">{@render sub()}</div>{/if}
  {@render children?.()}
</div>

<style>
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
  }
  /* keep the text above a chart drawn behind it */
  .l,
  .big,
  .s {
    position: relative;
    z-index: 1;
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
    font-size: 11px;
    color: var(--muted);
  }
  .s {
    display: flex;
    justify-content: space-between;
    gap: 6px;
    white-space: nowrap;
    font-size: 10.5px;
    color: var(--faint);
    font-family: var(--mono);
  }

  .zone {
    height: 76px;
    background: var(--inset);
  }
  .zone .stripe {
    width: 5px;
  }
  .zone .u {
    margin-left: 1px;
  }

  .fan {
    height: 94px;
  }
  .fan .u {
    margin-left: 2px;
  }

  /* disk temperature: bus on the left of the sub line, disk size on the right */
  .disk {
    display: flex;
    flex-direction: column;
    padding: 14px 16px;
  }
  .disk .l {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .disk .big {
    font-size: 30px;
  }
  .disk .u {
    font-size: 13px;
    margin-left: 1px;
  }
  .disk .s {
    margin-top: auto;
    font-size: 11px;
  }
  .disk.na .big {
    color: var(--faint);
  }
  /* on the phone the disk tile keeps the basic tile's size */
  .disk.compact {
    display: block;
    padding: 11px 13px;
  }
  .disk.compact .big {
    font-size: 24px;
  }
  .disk.compact .u {
    font-size: 11px;
  }
  .disk.compact .s {
    margin-top: 0;
    font-size: 10.5px;
  }
</style>
