<script lang="ts">
  import { themeStore } from '$lib/stores/theme.svelte'

  const DARK = {
    color1: '#d5d1cd91',
    color2: '#c9a87c91',
    color3: '#0e1524',
    color4: '#0e1528',
  }

  const LIGHT = {
    color1: '#8899bb',
    color2: '#7b8eb8',
    color3: '#dde4f0',
    color4: '#c8d4e8',
  }

  let theme = $derived(themeStore.dark ? DARK : LIGHT)

  let { withStar }: { withStar?: boolean } = $props()
</script>

<svg class="auth-bg" aria-hidden="true" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <pattern
      id="khatam"
      x="0"
      y="0"
      width="72"
      height="72"
      patternUnits="userSpaceOnUse"
    >
      <rect width="72" height="72" fill="transparent" />
      <polygon
        points="21,0 51,0 72,21 72,51 51,72 21,72 0,51 0,21"
        fill="none"
        stroke={theme.color1}
        stroke-width="0.6"
        opacity="0.18"
      />
      <polygon
        points="36,8 44,28 64,28 50,40 56,60 36,48 16,60 22,40 8,28 28,28"
        fill="none"
        stroke={theme.color1}
        stroke-width="0.5"
        opacity="0.14"
      />
      <polygon
        points="36,14 46,30 62,36 46,42 36,58 26,42 10,36 26,30"
        fill="none"
        stroke={theme.color2}
        stroke-width="0.4"
        opacity="0.10"
      />
      <polygon
        points="36,22 40,30 49,30 43,36 45,45 36,40 27,45 29,36 23,30 32,30"
        fill={theme.color1}
        fill-opacity="0.04"
        stroke={theme.color1}
        stroke-width="0.3"
        opacity="0.18"
      />
      <polygon points="0,0 3,3 0,6 -3,3" fill={theme.color2} opacity="0.20" />
      <polygon
        points="72,0 75,3 72,6 69,3"
        fill={theme.color2}
        opacity="0.20"
      />
      <polygon
        points="0,72 3,75 0,78 -3,75"
        fill={theme.color2}
        opacity="0.20"
      />
      <polygon
        points="72,72 75,75 72,78 69,75"
        fill={theme.color2}
        opacity="0.20"
      />
      <circle cx="36" cy="36" r="1.4" fill={theme.color1} opacity="0.22" />
    </pattern>

    <radialGradient id="fp-vignette" cx="50%" cy="50%" r="60%">
      <stop offset="0%" stop-color={theme.color3} stop-opacity="0" />
      <stop offset="100%" stop-color={theme.color3} stop-opacity="0.88" />
    </radialGradient>

    <filter id="fp-glow" x="-40%" y="-40%" width="180%" height="180%">
      <feGaussianBlur stdDeviation="6" result="b" />
      <feMerge><feMergeNode in="b" /><feMergeNode in="SourceGraphic" /></feMerge
      >
    </filter>
  </defs>

  <rect width="100%" height="100%" fill={theme.color3} />
  <rect width="100%" height="100%" fill="url(#khatam)" />
  <rect width="100%" height="100%" fill="url(#fp-vignette)" />

  {#if withStar}
    <g class="auth-star-group">
      <circle
        r="96"
        fill="none"
        stroke={theme.color1}
        stroke-width="0.7"
        opacity="0.18"
      />
      <circle
        r="72"
        fill="none"
        stroke={theme.color1}
        stroke-width="0.5"
        stroke-dasharray="3 6"
        opacity="0.14"
      />
      <polygon
        points="0,-68 18,-18 68,0 18,18 0,68 -18,18 -68,0 -18,-18"
        fill={theme.color4}
        fill-opacity="0.95"
        stroke={theme.color1}
        stroke-width="1.2"
        filter="url(#fp-glow)"
        opacity="0.55"
      />
      <polygon
        points="0,-46 12,-12 46,0 12,12 0,46 -12,12 -46,0 -12,-12"
        fill="none"
        stroke={theme.color2}
        stroke-width="0.6"
        opacity="0.30"
      />
      <polygon
        points="0,-28 20,-20 28,0 20,20 0,28 -20,20 -28,0 -20,-20"
        fill="none"
        stroke={theme.color1}
        stroke-width="0.5"
        opacity="0.22"
      />
    </g>
  {/if}
</svg>

<style>
  /* Center the star group at viewport midpoint */
  .auth-star-group {
    transform: translate(50%, 50%);
    animation: auth-star-drift 120s linear infinite;
    transform-origin: 0 0;
  }

  .auth-star-group circle:nth-child(2) {
    animation: auth-ring-counter 80s linear infinite;
    transform-origin: 0 0;
  }

  @keyframes auth-star-drift {
    from {
      transform: translate(50%, 50%) rotate(0deg);
    }
    to {
      transform: translate(50%, 50%) rotate(360deg);
    }
  }

  @keyframes auth-ring-counter {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(-360deg);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .auth-star-group,
    .auth-star-group circle:nth-child(2) {
      animation: none;
    }
  }
</style>
