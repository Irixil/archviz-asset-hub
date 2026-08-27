<script lang="ts">
  import { ZoomIn, ZoomOut, RotateCw, Maximize, Minimize } from '@lucide/svelte'
  import { onDestroy } from 'svelte'

  interface Props {
    zoomIn?: () => void
    zoomOut?: () => void
    rotateRight?: () => void
    fullscreenTarget?: HTMLElement | null
    show?: () => void
  }

  let {
    zoomIn,
    zoomOut,
    rotateRight,
    fullscreenTarget = null,
    show = $bindable(),
  }: Props = $props()

  let toolbarVisible = $state(true)
  let isFullscreen = $state(false)
  let hideTimer: ReturnType<typeof setTimeout> | null = null

  function resetTimer() {
    toolbarVisible = true
    if (hideTimer) clearTimeout(hideTimer)
    hideTimer = setTimeout(() => {
      toolbarVisible = false
    }, 2500)
  }

  $effect(() => {
    show = resetTimer
    resetTimer()
  })

  onDestroy(() => {
    if (hideTimer) clearTimeout(hideTimer)
  })

  function toggleFullscreen() {
    if (!fullscreenTarget) return
    if (!document.fullscreenElement) fullscreenTarget.requestFullscreen()
    else document.exitFullscreen()
  }

  $effect(() => {
    const handler = () => {
      isFullscreen = !!document.fullscreenElement
    }
    document.addEventListener('fullscreenchange', handler)
    return () => document.removeEventListener('fullscreenchange', handler)
  })
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="asset-preview-toolbar transition-opacity duration-300 {toolbarVisible
    ? 'opacity-100'
    : 'pointer-events-none opacity-0'}"
  onclick={(e) => e.stopPropagation()}
>
  <div
    class="flex items-center gap-2 rounded-xl border border-white/20 bg-black/70 px-4 py-2.5 shadow-xl backdrop-blur-sm"
  >
    <button
      class="rounded-lg p-1.5 text-white transition-colors hover:bg-white/10"
      title="Zoom out"
      onclick={(e) => {
        e.stopPropagation()
        zoomOut?.()
        resetTimer()
      }}
    >
      <ZoomOut class="h-4 w-4" />
    </button>

    <button
      class="rounded-lg p-1.5 text-white transition-colors hover:bg-white/10"
      title="Zoom in"
      onclick={(e) => {
        e.stopPropagation()
        zoomIn?.()
        resetTimer()
      }}
    >
      <ZoomIn class="h-4 w-4" />
    </button>

    <div class="h-5 w-px bg-white/20"></div>

    <button
      class="rounded-lg p-1.5 text-white transition-colors hover:bg-white/10"
      title="Rotate 90°"
      onclick={(e) => {
        e.stopPropagation()
        rotateRight?.()
        resetTimer()
      }}
    >
      <RotateCw class="h-4 w-4" />
    </button>

    <div class="h-5 w-px bg-white/20"></div>

    <button
      class="rounded-lg p-1.5 text-white transition-colors hover:bg-white/10"
      title={isFullscreen ? 'Exit fullscreen' : 'Fullscreen'}
      onclick={(e) => {
        e.stopPropagation()
        toggleFullscreen()
        resetTimer()
      }}
    >
      {#if isFullscreen}
        <Minimize class="h-4 w-4" />
      {:else}
        <Maximize class="h-4 w-4" />
      {/if}
    </button>
  </div>
</div>
