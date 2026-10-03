<!--
  Open-bottom arc gauge (CasaOS style): a 240° track, the arc grows with the value
  and takes the colour of its level; the name sits in the gap at the bottom.
-->
<script lang="ts">
  const {
    name,
    value,
    unit,
    fraction,
    color,
  }: {
    name: string
    /** Rounded number shown in the middle; undefined shows a dash. */
    value: number | undefined
    unit: string
    /** 0..1: how much of the track is filled. */
    fraction: number
    /** CSS colour of the arc. */
    color: string
  } = $props()

  const CIRC = 2 * Math.PI * 30 // radius 30 in a 80×80 viewBox
  const ARC = CIRC * (240 / 360) // visible track: 240°, 120° gap at the bottom
  const filled = $derived(Math.max(0, Math.min(1, fraction)) * ARC)
</script>

<div class="gauge">
  <svg viewBox="0 0 80 80" aria-hidden="true">
    <circle class="trk" cx="40" cy="40" r="30" transform="rotate(150 40 40)" style:stroke-dasharray="{ARC} {CIRC}" />
    <circle
      class="arc"
      cx="40"
      cy="40"
      r="30"
      transform="rotate(150 40 40)"
      style:stroke-dasharray="{filled} {CIRC}"
      style:stroke={color}
      style:opacity={value === undefined ? 0 : 1}
    />
  </svg>
  <div class="num"><span class="vv"><b>{value ?? '–'}</b>{#if value !== undefined}<i>{unit}</i>{/if}</span></div>
  <div class="gname">{name}</div>
</div>

<style>
  .gauge {
    position: relative;
    width: 100%;
    max-width: 132px;
    aspect-ratio: 1;
    margin: 0 auto;
  }
  svg {
    width: 100%;
    height: 100%;
    display: block;
  }
  .trk,
  .arc {
    fill: none;
    stroke-width: 9;
    stroke-linecap: round;
  }
  .trk {
    stroke: #33475b;
  }
  .arc {
    transition:
      stroke-dasharray 0.5s ease,
      stroke 0.4s ease;
  }
  .num {
    position: absolute;
    left: 0;
    right: 0;
    top: 50%;
    transform: translateY(-50%);
    text-align: center;
    line-height: 1;
  }
  .vv {
    position: relative;
    display: inline-block;
  }
  b {
    font-family: var(--mono);
    font-size: 21px;
    font-weight: 600;
    letter-spacing: -0.5px;
  }
  /* the unit hangs off the right of the number so the number itself stays centred */
  i {
    position: absolute;
    left: 100%;
    top: 1px;
    margin-left: 1.5px;
    font-family: var(--mono);
    font-size: 13px;
    font-weight: 600;
    font-style: normal;
    color: var(--muted);
  }
  .gname {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 8%;
    text-align: center;
    font-size: 12.5px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text);
    font-weight: 700;
  }
</style>
