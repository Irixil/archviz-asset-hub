<script lang="ts">
  import type { Asset } from '$lib/api'

  type Props = {
    src: string
    vars: string
    asset: Asset
  }

  let { src, vars, asset }: Props = $props()
</script>

<div
  class="zoom-overlay-bg fixed inset-0 z-40 w-screen bg-black/70 backdrop-blur-lg"
></div>
<div
  class="zoom-overlay fixed inset-0 z-42 grid w-[75%] place-items-center p-40"
  style={vars}
>
  <img
    {src}
    width={asset.width || 640}
    alt=""
    class="max-h-[80vh] max-w-3xl min-w-xl rounded-lg object-cover"
  />
</div>

<style lang="scss">
  .zoom-overlay-bg {
    animation: bg-fade-in 420ms cubic-bezier(0.16, 1, 0.3, 1) forwards;
  }

  .zoom-overlay {
    transform-origin: center center;
    will-change: transform, filter, opacity;
    animation: card-zoom-in 420ms cubic-bezier(0.16, 1, 0.3, 1) forwards;
  }

  @keyframes bg-fade-in {
    from {
      opacity: 0;
    }
    30% {
      opacity: 1;
    }
    85% {
      opacity: 1;
    }
    to {
      opacity: 0;
    }
  }

  @keyframes card-zoom-in {
    from {
      transform: translate(var(--tx), var(--ty)) scale(var(--sx), var(--sy));
      filter: brightness(1);
      opacity: 1;
    }
    /* peak: centered, slightly overshoots to 1.04, brief brightness flash */
    55% {
      transform: translate(0px, 0px) scale(1.04, 1.04);
      filter: brightness(1.25);
      opacity: 1;
    }
    /* settle: back to 1 */
    70% {
      transform: translate(0px, 0px) scale(1, 1);
      filter: brightness(1);
      opacity: 1;
    }
    /* fade as panel arrives */
    to {
      transform: translate(0px, 0px) scale(1, 1);
      filter: brightness(1);
      opacity: 0;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .zoom-overlay,
    .zoom-overlay-bg {
      animation: none;
      opacity: 0;
    }
  }
</style>
