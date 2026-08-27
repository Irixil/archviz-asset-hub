<script module lang="ts">
  export const ACTION_SHEET_KEY = Symbol('actionsheet')

  export interface ActionSheetItem {
    key: string
    label: string
    variant?: 'default' | 'danger' | 'disabled'
    onSelect: () => void
  }

  export interface ActionSheetContext {
    open: (config: {
      title?: string
      items: ActionSheetItem[]
      onClose?: () => void
    }) => void
    close: () => void
  }
</script>

<script lang="ts">
  import { setContext, type Snippet } from 'svelte'
  import { m } from '$lib/paraglide/messages'
  import { viewportStore } from '$lib/stores/viewport.svelte'

  interface Props {
    children?: Snippet
  }

  let { children }: Props = $props()

  let title = $state('')
  let items = $state<ActionSheetItem[]>([])
  let isOpen = $state(false)
  let currentY = $state(0)
  let dragging = $state(false)
  let startY = 0
  let onCloseCallback = $state<(() => void) | null>(null)

  const DISMISS_THRESHOLD = 80

  function runCloseCallback() {
    const callback = onCloseCallback
    onCloseCallback = null
    callback?.()
  }

  function closeSheet() {
    if (!isOpen) return
    isOpen = false
    currentY = 0
    dragging = false
    runCloseCallback()
  }

  setContext<ActionSheetContext>(ACTION_SHEET_KEY, {
    open(config) {
      title = config.title ?? ''
      items = config.items
      isOpen = true
      currentY = 0
      dragging = false
      onCloseCallback = config.onClose ?? null
    },
    close() {
      closeSheet()
    },
  })

  function handleSelect(item: ActionSheetItem) {
    if (item.variant === 'disabled') return
    const callback = item.onSelect
    closeSheet()
    setTimeout(() => callback(), viewportStore.isMobile ? 140 : 90)
  }

  function handlePointerDown(event: PointerEvent) {
    dragging = true
    startY = event.clientY
    currentY = 0
  }

  function handlePointerMove(event: PointerEvent) {
    if (!dragging) return
    currentY = Math.max(0, event.clientY - startY)
  }

  function handlePointerEnd() {
    if (!dragging) return
    dragging = false
    if (currentY > DISMISS_THRESHOLD) {
      closeSheet()
      return
    }
    currentY = 0
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') closeSheet()
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{@render children?.()}

{#if isOpen}
  <button
    type="button"
    class="sheet-scrim"
    onclick={closeSheet}
    aria-label={m.actions()}
  ></button>

  <div
    class="sheet-panel"
    role="dialog"
    aria-modal="true"
    aria-label={title || m.actions()}
    style={`transform: translateY(${currentY}px)`}
  >
    <div
      class="drag-region"
      role="presentation"
      onpointerdown={handlePointerDown}
      onpointermove={handlePointerMove}
      onpointerup={handlePointerEnd}
      onpointercancel={handlePointerEnd}
    >
      <div class="drag-handle" aria-hidden="true"></div>
      {#if title}
        <div class="sheet-title">{title}</div>
      {/if}
    </div>

    <ul class="sheet-items">
      {#each items as item (item.key)}
        <li>
          <button
            type="button"
            class="sheet-item"
            class:danger={item.variant === 'danger'}
            disabled={item.variant === 'disabled'}
            onclick={() => handleSelect(item)}
          >
            {item.label}
          </button>
        </li>
      {/each}
    </ul>

    <button type="button" class="sheet-cancel" onclick={closeSheet}>
      {m.cancel()}
    </button>
  </div>
{/if}

<style>
  .sheet-scrim {
    position: fixed;
    inset: 0;
    z-index: 80;
    border: 0;
    background: rgb(9 14 24 / 0.38);
    padding: 0;
  }

  .sheet-panel {
    position: fixed;
    right: 0;
    bottom: 0;
    left: 0;
    z-index: 90;
    border-radius: 18px 18px 0 0;
    background: var(--bg-surface);
    padding: 0 0 calc(env(safe-area-inset-bottom) + 0.75rem);
    box-shadow: 0 -18px 50px rgb(0 0 0 / 0.18);
    animation: sheet-enter 180ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  .drag-region {
    touch-action: none;
  }

  .drag-handle {
    margin: 0.75rem auto 0.5rem;
    height: 4px;
    width: 40px;
    border-radius: 999px;
    background: var(--border-default);
  }

  .sheet-title {
    padding: 0 1rem 0.75rem;
    color: var(--text-muted);
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .sheet-items {
    margin: 0;
    list-style: none;
    padding: 0;
  }

  .sheet-item,
  .sheet-cancel {
    display: block;
    width: 100%;
    border: 0;
    background: transparent;
    color: var(--text-primary);
    text-align: left;
    font-size: 0.95rem;
  }

  .sheet-item {
    min-height: 52px;
    border-top: 1px solid var(--border-subtle);
    padding: 0.95rem 1rem;
  }

  .sheet-item.danger {
    color: #c43d3d;
  }

  .sheet-cancel {
    margin-top: 0.5rem;
    border-top: 1px solid var(--border-subtle);
    padding: 0.95rem 1rem 0.35rem;
    font-weight: 700;
  }

  @keyframes sheet-enter {
    from {
      transform: translateY(100%);
    }
    to {
      transform: translateY(0);
    }
  }

  @media (min-width: 768px) {
    .sheet-scrim {
      background: transparent;
    }

    .sheet-panel {
      left: auto;
      width: min(320px, calc(100vw - 2rem));
      border-radius: 16px;
      margin: 0 1rem 1rem auto;
      padding-bottom: 0.5rem;
      box-shadow: 0 18px 42px rgb(0 0 0 / 0.16);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .sheet-panel {
      animation: none;
    }
  }
</style>
