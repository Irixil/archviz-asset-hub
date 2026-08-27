<script lang="ts">
  import { untrack } from 'svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetApi, type Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    asset: Asset
    creating: boolean
    handleCreate: (kind: string, params: Record<string, unknown>) => void
    initialParams?: Record<string, unknown> | null
  }

  let { asset, creating, handleCreate, initialParams = null }: Props = $props()

  const kind = 'image_crop'

  let cropFormat = $state<'jpeg' | 'png'>(
    untrack(() => (initialParams?.format as 'jpeg' | 'png') ?? 'png')
  )

  let img = $state<HTMLImageElement>()
  let imgLoaded = $state(false)

  let dispW = $state(0)
  let dispH = $state(0)
  let scaleX = $state(1)
  let scaleY = $state(1)

  let dragging = $state(false)
  let draggingHandle = $state<'tl' | 'tr' | 'bl' | 'br' | null>(null)
  let draggingBox = $state(false)
  let boxDragOrigin = $state<{
    x: number
    y: number
    startX: number
    startY: number
    endX: number
    endY: number
  } | null>(null)
  let startX = $state(0)
  let startY = $state(0)
  let endX = $state(0)
  let endY = $state(0)

  let box = $derived(
    endX !== startX || endY !== startY
      ? {
          x: Math.min(startX, endX),
          y: Math.min(startY, endY),
          w: Math.abs(endX - startX),
          h: Math.abs(endY - startY),
        }
      : null
  )

  let crop = $derived(
    box && scaleX > 0 && scaleY > 0
      ? {
          x: Math.round(box.x / scaleX),
          y: Math.round(box.y / scaleY),
          width: Math.round(box.w / scaleX),
          height: Math.round(box.h / scaleY),
        }
      : null
  )

  function updateScale() {
    if (!img) return
    dispW = img.offsetWidth
    dispH = img.offsetHeight
    scaleX = dispW / img.naturalWidth
    scaleY = dispH / img.naturalHeight
  }

  function onImgLoad() {
    updateScale()
    imgLoaded = true
    if (initialParams) {
      const x = initialParams.x as number
      const y = initialParams.y as number
      const w = initialParams.width as number
      const h = initialParams.height as number
      if ([x, y, w, h].every((n) => typeof n === 'number')) {
        startX = x * scaleX
        startY = y * scaleY
        endX = (x + w) * scaleX
        endY = (y + h) * scaleY
      }
    }
  }

  function pos(e: MouseEvent | TouchEvent | PointerEvent): {
    x: number
    y: number
  } {
    if (!img) return { x: 0, y: 0 }
    const rect = img.getBoundingClientRect()
    const client = 'touches' in e ? (e as TouchEvent).touches[0] : e
    return {
      x: Math.max(0, Math.min(client.clientX - rect.left, rect.width)),
      y: Math.max(0, Math.min(client.clientY - rect.top, rect.height)),
    }
  }

  function onDown(e: MouseEvent | TouchEvent) {
    if (!imgLoaded) return
    e.preventDefault()
    updateScale()
    const p = pos(e)
    startX = p.x
    startY = p.y
    endX = p.x
    endY = p.y
    dragging = true
  }

  function onMove(e: MouseEvent | TouchEvent) {
    if (!dragging) return
    e.preventDefault()
    const p = pos(e)
    endX = p.x
    endY = p.y
  }

  function onUp(e: MouseEvent | TouchEvent) {
    if (!dragging) return
    e.preventDefault()
    dragging = false
  }

  const handleCursors: Record<string, string> = {
    tl: 'nwse-resize',
    tr: 'nesw-resize',
    bl: 'nesw-resize',
    br: 'nwse-resize',
  }

  function onHandleDown(e: PointerEvent, handle: 'tl' | 'tr' | 'bl' | 'br') {
    e.stopPropagation()
    e.preventDefault()
    draggingHandle = handle
    ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
  }

  function onHandleMove(e: PointerEvent) {
    if (!draggingHandle) return
    e.preventDefault()
    const p = pos(e)
    if (draggingHandle === 'tl') {
      startX = p.x
      startY = p.y
    } else if (draggingHandle === 'tr') {
      endX = p.x
      startY = p.y
    } else if (draggingHandle === 'bl') {
      startX = p.x
      endY = p.y
    } else if (draggingHandle === 'br') {
      endX = p.x
      endY = p.y
    }
  }

  function onHandleUp() {
    draggingHandle = null
  }

  function onBoxDown(e: PointerEvent) {
    if (!box) return
    e.stopPropagation()
    e.preventDefault()
    draggingBox = true
    const p = pos(e)
    boxDragOrigin = {
      x: p.x,
      y: p.y,
      startX: box.x,
      startY: box.y,
      endX: box.x + box.w,
      endY: box.y + box.h,
    }
    ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
  }

  function onBoxMove(e: PointerEvent) {
    if (!draggingBox || !boxDragOrigin) return
    e.preventDefault()
    const p = pos(e)
    const dx = p.x - boxDragOrigin.x
    const dy = p.y - boxDragOrigin.y
    const w = boxDragOrigin.endX - boxDragOrigin.startX
    const h = boxDragOrigin.endY - boxDragOrigin.startY
    const nx = Math.max(0, Math.min(boxDragOrigin.startX + dx, dispW - w))
    const ny = Math.max(0, Math.min(boxDragOrigin.startY + dy, dispH - h))
    startX = nx
    startY = ny
    endX = nx + w
    endY = ny + h
  }

  function onBoxUp() {
    draggingBox = false
    boxDragOrigin = null
  }
</script>

<div class="space-y-5">
  <div class="canvas-area">
    <div class="relative">
      <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
      <img
        bind:this={img}
        src={assetApi.fileUrl(asset.id)}
        alt="Preview"
        class="block max-h-[480px] w-full cursor-crosshair rounded object-contain"
        draggable="false"
        onload={onImgLoad}
        onmousedown={onDown}
        onmousemove={onMove}
        onmouseup={onUp}
        onmouseleave={onUp}
        ontouchstart={onDown}
        ontouchmove={onMove}
        ontouchend={onUp}
      />

      {#if box && box.w > 2 && box.h > 2 && img}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <svg
          class="absolute inset-0 rounded"
          style="width:{dispW}px; height:{dispH}px;"
          onpointermove={onHandleMove}
          onpointerup={onHandleUp}
        >
          <defs>
            <mask id="crop-mask">
              <rect width="100%" height="100%" fill="white" />
              <rect
                x={box.x}
                y={box.y}
                width={box.w}
                height={box.h}
                fill="black"
              />
            </mask>
          </defs>
          <rect
            width="100%"
            height="100%"
            fill="rgba(0,0,0,0.45)"
            mask="url(#crop-mask)"
            style="pointer-events:none"
          />
          <rect
            x={box.x}
            y={box.y}
            width={box.w}
            height={box.h}
            fill="none"
            stroke="white"
            stroke-width="1.5"
            style="pointer-events:none"
          />
          {#each [1, 2] as t}
            <line
              x1={box.x + (box.w * t) / 3}
              y1={box.y}
              x2={box.x + (box.w * t) / 3}
              y2={box.y + box.h}
              stroke="rgba(255,255,255,0.4)"
              stroke-width="1"
              style="pointer-events:none"
            />
            <line
              x1={box.x}
              y1={box.y + (box.h * t) / 3}
              x2={box.x + box.w}
              y2={box.y + (box.h * t) / 3}
              stroke="rgba(255,255,255,0.4)"
              stroke-width="1"
              style="pointer-events:none"
            />
          {/each}
          <rect
            x={box.x}
            y={box.y}
            width={box.w}
            height={box.h}
            fill="transparent"
            style="cursor:move"
            onpointerdown={onBoxDown}
            onpointermove={onBoxMove}
            onpointerup={onBoxUp}
          />
          {#each [['tl', box.x, box.y], ['tr', box.x + box.w, box.y], ['bl', box.x, box.y + box.h], ['br', box.x + box.w, box.y + box.h]] as [handle, cx, cy]}
            <!-- invisible larger hit area -->
            <rect
              x={+cx - 8}
              y={+cy - 8}
              width="16"
              height="16"
              fill="transparent"
              style="cursor:{handleCursors[handle as string]}"
              onpointerdown={(e) =>
                onHandleDown(e, handle as 'tl' | 'tr' | 'bl' | 'br')}
              onpointermove={onHandleMove}
              onpointerup={onHandleUp}
            />
            <!-- visible indicator -->
            <rect
              x={+cx - 4}
              y={+cy - 4}
              width="8"
              height="8"
              fill="white"
              rx="1"
              style="pointer-events:none"
            />
          {/each}
        </svg>
      {/if}
    </div>
  </div>

  <p class="crop-hint">
    {#if crop}
      {crop.x}, {crop.y} — {crop.width} × {crop.height} px
    {:else}
      Draw a selection on the image
    {/if}
  </p>

  <div>
    <label for="crop-format" class="field-label">{m.format()}</label>
    <div id="crop-format" class="flex gap-2">
      {#each ['jpeg', 'png'] as fmt}
        <button
          type="button"
          class="toggle-btn flex-1 {cropFormat === fmt ? 'active' : ''}"
          onclick={() => {
            cropFormat = fmt as typeof cropFormat
          }}>{fmt.toUpperCase()}</button
        >
      {/each}
    </div>
  </div>

  <Button
    disabled={creating ||
      authStore.role === 'viewer' ||
      !crop ||
      crop.width < 1 ||
      crop.height < 1}
    onclick={() =>
      crop && handleCreate(kind, { ...crop, format: cropFormat, quality: 85 })}
    class="w-full"
  >
    {creating ? m.queuing_() : m.variant_create_crop()}
  </Button>
</div>

<style>
  .canvas-area {
    display: flex;
    justify-content: center;
    min-height: 160px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--bg-app);
    padding: 12px;
    user-select: none;
  }
  .crop-hint {
    font-size: 0.8125rem;
    color: var(--text-muted);
  }
  .field-label {
    display: block;
    margin-bottom: 4px;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-secondary);
  }
  .toggle-btn {
    border-radius: 7px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    color: var(--text-secondary);
    padding: 7px 0;
    font-size: 0.8125rem;
    font-weight: 500;
    transition: all 0.1s ease;
    cursor: pointer;
  }
  .toggle-btn:hover {
    border-color: var(--accent-cta);
    color: var(--text-primary);
  }
  .toggle-btn.active {
    border-color: var(--accent-cta);
    background: oklch(93% 0.04 270);
    color: oklch(40% 0.18 270);
  }
  :global(.dark) .toggle-btn.active {
    background: oklch(30% 0.08 270 / 0.4);
    color: oklch(78% 0.12 270);
  }
</style>
