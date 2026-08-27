<script lang="ts">
  import { ZoomIn, ZoomOut, Keyboard } from '@lucide/svelte'
  import { statusBarStore as s } from '$lib/stores/bottomStatusBar.svelte'
  import Hint from './ui/Hint.svelte'
  import ThemeToggle from './ThemeToggle.svelte'
  import { gModeActive } from '$lib/shortcuts/sequence'
  import { triggerAction } from '$lib/shortcuts'
  import { fade } from 'svelte/transition'
</script>

<div
  class="relative z-10 flex h-10 shrink-0 items-center justify-between border-t border-[var(--border-subtle)] bg-[var(--bg-surface)] px-4"
>
  {#if $gModeActive}
    <div
      transition:fade={{ duration: 100 }}
      class="flex items-center gap-1 text-xs text-zinc-500 dark:text-zinc-400"
    >
      <kbd
        class="rounded border border-zinc-300 bg-zinc-100 px-1 py-px font-mono dark:border-zinc-600 dark:bg-zinc-800"
        >g</kbd
      >
      <span>→</span>
      <span><kbd class="font-mono">l</kbd>素材库</span>
      <span class="text-zinc-300 dark:text-zinc-600">·</span>
      <span><kbd class="font-mono">p</kbd>项目</span>
      <span class="text-zinc-300 dark:text-zinc-600">·</span>
      <span><kbd class="font-mono">t</kbd>标签</span>
      <span class="text-zinc-300 dark:text-zinc-600">·</span>
      <span><kbd class="font-mono">s</kbd>设置</span>
    </div>
  {:else if s.slot1}
    <Hint class="text-sm">
      {s.slot1}
    </Hint>
  {:else}
    <span></span>
  {/if}

  {#if s.showZoom}
    <div class="mr-4 ml-auto flex items-center gap-1 text-[var(--text-muted)]">
      <button
        type="button"
        class="status-icon"
        aria-label="缩小素材卡片"
        onclick={() => s.zoomDecrease()}
      >
        <ZoomOut class="h-4 w-4" />
      </button>
      <input
        class="w-28 accent-[var(--accent)]"
        type="range"
        min="0"
        max={s.sliderMax}
        bind:value={s.zoom}
        aria-label="素材卡片大小"
      />
      <button
        type="button"
        class="status-icon"
        aria-label="放大素材卡片"
        onclick={() => s.zoomIncrease()}
      >
        <ZoomIn class="h-4 w-4" />
      </button>
    </div>
  {/if}

  <div class="flex items-center gap-3">
    <button
      type="button"
      class="status-icon"
      aria-label="打开快捷键帮助"
      onclick={() => triggerAction('help.toggle')}
    >
      <Keyboard class="h-3.5 w-3.5" />
    </button>

    <ThemeToggle />
  </div>
</div>

<style>
  .status-icon {
    display: inline-flex;
    width: 2rem;
    height: 2rem;
    align-items: center;
    justify-content: center;
    border-radius: 0.25rem;
    color: var(--text-muted);
  }
  .status-icon:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
</style>
