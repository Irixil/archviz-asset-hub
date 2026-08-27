<script lang="ts">
  import type {
    WorkflowGraphNode,
    WorkflowNodeSchema,
  } from '$lib/api/workflows'
  import { assetApi } from '$lib/api/assets'
  import { variantApi } from '$lib/api/client'

  interface Props {
    node: WorkflowGraphNode
    schema: WorkflowNodeSchema | null
    selected?: boolean
    readonly?: boolean
    status?: string | null
    inputCtx?: Record<string, unknown> | null
    outputCtx?: Record<string, unknown> | null
    onSelect?: () => void
    onMove?: (dx: number, dy: number) => void
    onPortDown?: (portId: string, event: PointerEvent) => void
    onPortUp?: (portId: string) => void
    onPortContextMenu?: (
      portId: string,
      isOutput: boolean,
      event: MouseEvent
    ) => void
  }

  let {
    node,
    schema,
    selected = false,
    readonly = false,
    status = null,
    inputCtx = null,
    outputCtx = null,
    onSelect = () => {},
    onMove = () => {},
    onPortDown = () => {},
    onPortUp = () => {},
    onPortContextMenu = () => {},
  }: Props = $props()

  const assetThumbUrl = $derived.by(() => {
    const assetId = inputCtx?.['asset_id']
    if (typeof assetId !== 'string' || !assetId) return null
    return assetApi.thumbUrl(assetId)
  })

  const variantThumbUrl = $derived.by(() => {
    const assetId = outputCtx?.['asset_id'] ?? inputCtx?.['asset_id']
    const variantId = outputCtx?.['variant_id']
    if (typeof variantId !== 'string' || !variantId) return null
    if (typeof assetId !== 'string' || !assetId) return null
    return variantApi.thumbUrl(assetId, variantId)
  })

  function accentColor(category: string | undefined) {
    switch (category) {
      case 'trigger':
        return '#0ea5e9'
      case 'filter':
        return '#f59e0b'
      case 'action':
        return '#10b981'
      default:
        return '#64748b'
    }
  }

  function accentBorder(category: string | undefined) {
    switch (category) {
      case 'trigger':
        return 'border-sky-400/60'
      case 'filter':
        return 'border-amber-400/60'
      case 'action':
        return 'border-emerald-400/60'
      default:
        return 'border-slate-400/40'
    }
  }

  function statusRing(state: string | null) {
    switch (state) {
      case 'running':
        return 'ring-2 ring-sky-500/60'
      case 'completed':
        return 'ring-2 ring-emerald-500/60'
      case 'failed':
        return 'ring-2 ring-rose-500/70'
      default:
        return ''
    }
  }

  function statusBadge(state: string) {
    switch (state) {
      case 'failed':
        return 'bg-rose-500/10 text-rose-700 dark:text-rose-300'
      case 'completed':
        return 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
      default:
        return 'bg-sky-500/10 text-sky-700 dark:text-sky-300'
    }
  }

  function portDot(category: string | undefined, isOutput: boolean) {
    if (!isOutput)
      return 'border-slate-400/50 bg-[var(--bg-elevated)] hover:bg-[var(--bg-hover)]'
    switch (category) {
      case 'trigger':
        return 'border-sky-400/60 bg-sky-400/20 hover:bg-sky-400/40'
      case 'filter':
        return 'border-amber-400/60 bg-amber-400/20 hover:bg-amber-400/40'
      case 'action':
        return 'border-emerald-400/60 bg-emerald-400/20 hover:bg-emerald-400/40'
      default:
        return 'border-slate-400/50 bg-[var(--bg-elevated)] hover:bg-[var(--bg-hover)]'
    }
  }

  let isDragging = $state(false)

  function handlePointerDown(event: PointerEvent) {
    if (readonly) return
    let lastX = event.clientX
    let lastY = event.clientY
    const target = event.currentTarget as HTMLElement
    target.setPointerCapture(event.pointerId)

    const move = (next: PointerEvent) => {
      isDragging = true
      onMove(next.clientX - lastX, next.clientY - lastY)
      lastX = next.clientX
      lastY = next.clientY
    }
    const end = () => {
      isDragging = false
      target.removeEventListener('pointermove', move)
      target.removeEventListener('pointerup', end)
      target.removeEventListener('pointercancel', end)
    }

    target.addEventListener('pointermove', move)
    target.addEventListener('pointerup', end)
    target.addEventListener('pointercancel', end)
  }

  const borderClass = $derived(
    selected ? accentBorder(schema?.category) : 'border-[var(--border-subtle)]'
  )
  const color = $derived(accentColor(schema?.category))
  const dotGlow = $derived(`0 0 6px ${color}80`)
  const cardTransition =
    'box-shadow 0.15s ease-out, border-color 0.15s ease-out, opacity 0.1s ease-out'
</script>

<div
  data-node-type={node.type}
  class="absolute w-[248px] rounded-[18px] border bg-[var(--bg-surface)] {borderClass} {statusRing(
    status
  )} {selected
    ? 'shadow-[0_8px_32px_rgba(15,23,42,0.22)]'
    : 'shadow-[0_2px_8px_rgba(15,23,42,0.06)]'} {isDragging
    ? 'scale-[1.01] cursor-grabbing opacity-90'
    : ''}"
  style:left={`${node.position.x}px`}
  style:top={`${node.position.y}px`}
  style:transition={cardTransition}
>
  {#if assetThumbUrl || variantThumbUrl}
    <div
      class="pointer-events-none absolute bottom-[calc(100%+10px)] left-0 flex gap-2"
    >
      {#if assetThumbUrl}
        <div class="flex flex-col items-start gap-1">
          <span
            class="rounded-full border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-2 py-0.5 font-mono text-[9px] font-semibold tracking-[0.1em] text-[var(--text-muted)] uppercase"
            >asset</span
          >
          <div
            class="h-14 w-14 overflow-hidden rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-elevated)] shadow-[0_4px_16px_rgba(15,23,42,0.18)]"
          >
            <img
              src={assetThumbUrl}
              alt="Asset thumbnail"
              class="h-full w-full object-cover"
              loading="lazy"
            />
          </div>
        </div>
      {/if}
      {#if variantThumbUrl}
        <div class="flex flex-col items-start gap-1">
          <span
            class="rounded-full border border-emerald-400/30 bg-[var(--bg-elevated)] px-2 py-0.5 font-mono text-[9px] font-semibold tracking-[0.1em] text-emerald-600 uppercase dark:text-emerald-400"
            >variant</span
          >
          <div
            class="h-14 w-14 overflow-hidden rounded-xl border border-emerald-400/30 bg-[var(--bg-elevated)] shadow-[0_4px_16px_rgba(15,23,42,0.18)]"
          >
            <img
              src={variantThumbUrl}
              alt="Variant thumbnail"
              class="h-full w-full object-cover"
              loading="lazy"
            />
          </div>
        </div>
      {/if}
    </div>
  {/if}

  <button
    type="button"
    class="flex w-full items-start justify-between gap-3 rounded-t-[18px] px-4 py-3.5 text-left {readonly
      ? ''
      : 'cursor-grab active:cursor-grabbing'}"
    onclick={() => onSelect()}
    onpointerdown={handlePointerDown}
  >
    <div class="min-w-0 flex-1">
      <div class="flex items-center gap-2.5">
        <span
          class="h-2 w-2 shrink-0 rounded-full"
          style:background-color={color}
          style:box-shadow={dotGlow}
        ></span>
        <p class="truncate text-sm font-semibold text-[var(--text-primary)]">
          {schema?.label ?? node.type}
        </p>
      </div>
    </div>

    {#if status}
      <span
        class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase {statusBadge(
          status
        )}"
      >
        {status}
      </span>
    {/if}
  </button>

  {#if schema && (schema.inputs.length > 0 || schema.outputs.length > 0)}
    <div class="border-t border-[var(--border-subtle)] px-3 py-3">
      <div class="relative flex items-start justify-between gap-4">
        <!-- Input ports (left) -->
        <div class="space-y-[10px]">
          {#each schema.inputs as port (port.id)}
            <div class="flex items-center gap-1.5">
              <button
                type="button"
                aria-label={`Connect input: ${port.label}`}
                title={`Connect input: ${port.label}`}
                class="h-2.5 w-2.5 rounded-full border-[1.5px] transition-all {portDot(
                  schema?.category,
                  false
                )} {readonly ? '' : 'cursor-crosshair hover:scale-125'}"
                disabled={readonly}
                onpointerup={() => onPortUp(port.id)}
                oncontextmenu={(e) => {
                  e.preventDefault()
                  onPortContextMenu(port.id, false, e)
                }}
              ></button>
              <span
                class="text-[10px] font-medium tracking-[0.12em] text-[var(--text-muted)] uppercase"
                >{port.label}</span
              >
            </div>
          {/each}
        </div>

        <!-- Output ports (right) -->
        <div class="flex flex-col items-end space-y-[10px]">
          {#each schema.outputs as port (port.id)}
            <div class="flex items-center gap-1.5">
              <span
                class="text-[10px] font-medium tracking-[0.12em] text-[var(--text-muted)] uppercase"
                >{port.label}</span
              >
              <button
                type="button"
                aria-label={`Draw edge from: ${port.label}`}
                title={`Draw edge from: ${port.label}`}
                class="h-2.5 w-2.5 rounded-full border-[1.5px] transition-all {portDot(
                  schema?.category,
                  true
                )} {readonly ? '' : 'cursor-crosshair hover:scale-125'}"
                disabled={readonly}
                onpointerdown={(event) => onPortDown(port.id, event)}
                oncontextmenu={(e) => {
                  e.preventDefault()
                  onPortContextMenu(port.id, true, e)
                }}
              ></button>
            </div>
          {/each}
        </div>
      </div>
    </div>
  {/if}
</div>
