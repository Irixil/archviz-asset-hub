<script lang="ts">
  import type { Asset, PublicAsset } from '$lib/api'
  import AssetIcon from '$lib/components/asset/AssetIcon.svelte'

  type Props = {
    asset: Asset | PublicAsset
    category: 'document' | 'audio' | 'video' | 'image'
    thumbUrl: string
    assetUrl: string
    zoomIn?: () => void
    zoomOut?: () => void
    zoomReset?: () => void
    onwheel?: (e: WheelEvent) => void
    rotateRight?: () => void
  }

  let {
    category,
    asset,
    thumbUrl,
    assetUrl,
    zoomIn = $bindable(),
    zoomOut = $bindable(),
    zoomReset = $bindable(),
    onwheel = $bindable(),
    rotateRight = $bindable(),
  }: Props = $props()
  let isPdf = $derived(asset.mime_type.includes('/pdf'))
  let haveSplashImage = $derived(
    thumbUrl && (category === 'audio' || (category === 'document' && !isPdf))
  )

  let audioEl: HTMLAudioElement | undefined = $state()
  let audioProgress = $state(0)

  function onAudioTimeUpdate() {
    if (audioEl && audioEl.duration) {
      audioProgress = audioEl.currentTime / audioEl.duration
    }
  }

  // Progressive image load: show thumb immediately, crossfade to full once loaded
  let fullLoaded = $state(false)
  let fullSrc = $state('')
  let splashAlone = $derived(
    !(isPdf || category === 'video' || category === 'audio' || !thumbUrl)
  )

  $effect(() => {
    // Reset on asset change
    fullLoaded = false
    fullSrc = ''
    if (category !== 'image') return
    const url = assetUrl
    const img = new Image()
    img.onload = () =>
      setTimeout(() => {
        fullSrc = url
        setTimeout(() => (fullLoaded = true), 10)
      }, 10)
    img.src = url
    return () => {
      img.onload = null
    }
  })

  const MIN_SCALE = 1
  const MAX_SCALE = 5
  const STEP = 0.05

  let scale = $state(1)
  let translateX = $state(0)
  let translateY = $state(0)
  let rotation = $state(0)
  let animated = $state(true)
  let container: HTMLDivElement

  // reset zoom and rotation on asset navigation (no animation)
  $effect(() => {
    asset.id
    animated = false
    scale = 1
    translateX = 0
    translateY = 0
    rotation = 0
    setTimeout(() => {
      animated = true
    }, 50)
  })

  function applyZoom(newScale: number, originX: number, originY: number) {
    newScale = Math.min(MAX_SCALE, Math.max(MIN_SCALE, newScale))
    const ratio = newScale / scale
    translateX = originX - ratio * (originX - translateX)
    translateY = originY - ratio * (originY - translateY)
    scale = newScale
  }

  let wheelTimer: ReturnType<typeof setTimeout> | null = null
  function onWheel(e: WheelEvent) {
    if (!container) return
    e.preventDefault()
    animated = false
    if (wheelTimer) clearTimeout(wheelTimer)
    wheelTimer = setTimeout(() => {
      animated = true
    }, 150)
    const rect = container.getBoundingClientRect()
    applyZoom(
      scale + (e.deltaY > 0 ? -STEP : STEP),
      e.clientX - rect.left,
      e.clientY - rect.top
    )
  }

  function centerZoom(delta: number) {
    if (!container) return
    const { width, height } = container.getBoundingClientRect()
    applyZoom(scale + delta, width / 2, height / 2)
  }

  // Expose center-zoom, rotate, and wheel handler to parent via bindable refs
  $effect(() => {
    zoomIn = () => centerZoom(STEP)
    zoomOut = () => centerZoom(-STEP)
    zoomReset = () => {
      scale = 1
      translateX = 0
      translateY = 0
    }
    onwheel = onWheel
    rotateRight = () => {
      rotation = (rotation + 90) % 360
    }
  })
</script>

<div
  bind:this={container}
  class="asset-preview-full max-h-[80vh] max-w-3xl min-w-xl flex-1 items-center justify-center"
  style="transform: scale({scale}) rotate({rotation}deg); transform-origin: center; transition: {animated
    ? 'transform 0.25s cubic-bezier(0.25, 0.46, 0.45, 0.94)'
    : 'none'}; cursor: {scale > 1 ? 'grab' : 'default'};"
>
  {#if category === 'image'}
    <div class="image-crossfade pointer-events-none">
      <!-- full-res: layout anchor, drives container size via width/height attrs -->
      <img
        src={fullSrc || thumbUrl}
        alt={asset.original_filename}
        data-asset-dynamic-resource={asset.id}
        width={'width' in asset && asset.width ? asset.width : undefined}
        height={'height' in asset && asset.height ? asset.height : undefined}
        class="anchor"
        onerror={(e) => {
          ;(e.currentTarget as HTMLImageElement).style.display = 'none'
        }}
      />
      <!-- thumb: absolute overlay, fades out once full-res is ready -->
      <img
        src={thumbUrl}
        alt={asset.original_filename}
        class="thumb-overlay"
        style="opacity: {fullLoaded ? 0 : 1};"
      />
    </div>
  {:else if haveSplashImage}
    <img
      src={thumbUrl}
      alt={asset.original_filename}
      data-asset-dynamic-resource={asset.id}
      class="{!splashAlone
        ? 'asset-splash'
        : ''} min-w-xl object-cover {category === 'audio' ? 'invert' : ''}"
      style={category === 'audio' ? `--progress: ${audioProgress}` : undefined}
      loading="lazy"
      onerror={(e) => {
        ;(e.currentTarget as HTMLImageElement).style.display = 'none'
      }}
    />
  {/if}
  {#if isPdf}
    <iframe
      class="min-h-[80vh] w-full min-w-3xl"
      src={assetUrl}
      title={asset.original_filename}
    ></iframe>
  {:else if category === 'video'}
    <video class="max-h-[80vh] w-full object-contain" controls>
      <source
        data-asset-dynamic-resource={asset.id}
        src={assetUrl}
        type={asset.mime_type}
      />
      Your browser does not support the video tag.
    </video>
  {:else if category === 'audio'}
    <audio
      class="w-full"
      controls
      bind:this={audioEl}
      ontimeupdate={onAudioTimeUpdate}
    >
      <source
        data-asset-dynamic-resource={asset.id}
        src={assetUrl}
        type={asset.mime_type}
      />
      Your browser does not support the audio element.
    </audio>
  {:else if !thumbUrl}
    <div class="flex h-full items-center justify-center">
      <div
        class="flex h-12 w-12 items-center justify-center rounded-xl bg-white/25"
      >
        <AssetIcon {category} class="h-7 w-7 text-white" />
      </div>
    </div>
  {/if}
</div>

<style>
  .image-crossfade {
    position: relative;
    display: inline-block;
    max-width: 100%;
    max-height: 80vh;
  }

  /* layout anchor: full-res image, sizes the container */
  .image-crossfade .anchor {
    display: block;
    max-width: 100%;
    max-height: 80vh;
    object-fit: contain;
  }

  /* thumb: absolute overlay covering the anchor exactly, fades out on load */
  .image-crossfade .thumb-overlay {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: contain;
    transition: opacity 400ms cubic-bezier(0.16, 1, 0.3, 1);
  }

  @media (prefers-reduced-motion: reduce) {
    .image-crossfade .thumb-overlay {
      transition: none;
    }
  }

  .asset-splash {
    mask-image: linear-gradient(
      to right,
      black calc(var(--progress, 0) * 100%),
      rgba(0, 0, 0, 0.3) calc(var(--progress, 0) * 100%)
    );
    mask-mode: alpha;
    transition: mask-image 0.1s linear;
  }

  @media (prefers-reduced-motion: reduce) {
    .asset-splash {
      transition: none;
    }
  }
</style>
