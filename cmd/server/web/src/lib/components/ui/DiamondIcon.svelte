<script lang="ts">
  interface Props {
    letter: string
    color?: string
    bright?: string
    size?: number
    class?: string
  }

  let {
    letter,
    color = '#4f46e5',
    bright = '#a5b4fc',
    size = 32,
    class: extraClass = '',
  }: Props = $props()

  const filterId = `diam-glow-${Math.random().toString(36).slice(2)}`

  // Diamond half-size in SVG units (viewBox is size×size, diamond centered)
  // svelte-ignore state_referenced_locally
  const half = size * 0.47
  const inner = half * 0.52

  // Lines from outer diamond midpoints to inner diamond midpoints
  const facets: [number, number, number, number][] = [
    [-half, 0, -inner, 0],
    [0, -half, 0, -inner],
    [half, 0, inner, 0],
    [0, half, 0, inner],
    [-half * 0.72, -half * 0.72, -inner * 0.72, -inner * 0.72],
    [half * 0.72, -half * 0.72, inner * 0.72, -inner * 0.72],
    [half * 0.72, half * 0.72, inner * 0.72, inner * 0.72],
    [-half * 0.72, half * 0.72, -inner * 0.72, inner * 0.72],
  ]
</script>

<svg
  width={size}
  height={size}
  viewBox="{-size / 2} {-size / 2} {size} {size}"
  class={extraClass}
  aria-hidden="true"
>
  <defs>
    <filter id={filterId} x="-60%" y="-60%" width="220%" height="220%">
      <feGaussianBlur in="SourceGraphic" stdDeviation="2.5" result="blur" />
      <feMerge>
        <feMergeNode in="blur" />
        <feMergeNode in="SourceGraphic" />
      </feMerge>
    </filter>
  </defs>

  <!-- Outer diamond with glow -->
  <polygon
    points="0,{-half} {half},0 0,{half} {-half},0"
    fill={color}
    fill-opacity="0.88"
    stroke={bright}
    stroke-width="1.4"
    filter="url(#{filterId})"
  />

  <!-- Inner diamond highlight -->
  <polygon
    points="0,{-inner} {inner},0 0,{inner} {-inner},0"
    fill={bright}
    fill-opacity="0.15"
    stroke={bright}
    stroke-width="0.7"
    stroke-opacity="0.6"
  />

  <!-- Facet lines -->
  {#each facets as [x1, y1, x2, y2]}
    <line
      {x1}
      {y1}
      {x2}
      {y2}
      stroke={bright}
      stroke-width="0.5"
      stroke-opacity="0.45"
    />
  {/each}

  <!-- Letter -->
  <text
    x="0"
    y={size * 0.075}
    text-anchor="middle"
    dominant-baseline="middle"
    fill="white"
    font-size={size * 0.34}
    font-weight="700"
    font-family="system-ui, sans-serif"
    letter-spacing="0">{letter}</text
  >
</svg>
