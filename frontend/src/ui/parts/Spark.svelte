<!--
  A canvas that fills its (positioned) parent and redraws on every live push and
  whenever its size changes. `draw` receives a cleared context in CSS pixels; it
  runs inside an effect, so state it reads also triggers a redraw.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import { prepare } from '../../lib/charts/canvas'
  import { dashboard } from '../../lib/state/dashboard.svelte'

  const {
    draw,
    left = '0px',
  }: {
    draw: (g: CanvasRenderingContext2D, w: number, h: number) => void
    /** Distance kept free on the left (e.g. a tile's colour stripe). */
    left?: string
  } = $props()

  let canvas = $state<HTMLCanvasElement>()
  let resized = $state(0)

  onMount(() => {
    const ro = new ResizeObserver(() => resized++)
    ro.observe(canvas!)
    return () => ro.disconnect()
  })

  $effect(() => {
    void dashboard.live.tick // a new push
    void resized // a new size
    if (!canvas) return
    const surface = prepare(canvas)
    if (surface) draw(surface.g, surface.w, surface.h)
  })
</script>

<canvas bind:this={canvas} style:left style:width="calc(100% - {left})"></canvas>

<style>
  canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    display: block;
  }
</style>
