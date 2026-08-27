type RubberBand = {
  startX: number
  startY: number
  x: number
  y: number
  w: number
  h: number
}

export function createRubberBand(getContainer: () => HTMLElement | undefined) {
  let rubberBand = $state<RubberBand | null>(null)

  function onMouseDown(e: MouseEvent) {
    if (!e.shiftKey || e.button !== 0) return
    if ((e.target as Element).closest('[data-asset-id]')) return
    e.preventDefault()
    rubberBand = {
      startX: e.clientX,
      startY: e.clientY,
      x: e.clientX,
      y: e.clientY,
      w: 0,
      h: 0,
    }
  }

  function onMouseMove(e: MouseEvent) {
    if (!rubberBand) return
    const x = Math.min(e.clientX, rubberBand.startX)
    const y = Math.min(e.clientY, rubberBand.startY)
    const w = Math.abs(e.clientX - rubberBand.startX)
    const h = Math.abs(e.clientY - rubberBand.startY)
    rubberBand = { ...rubberBand, x, y, w, h }
  }

  function onMouseUp(onSelect: (ids: string[]) => void) {
    if (!rubberBand) {
      rubberBand = null
      return
    }
    const container = getContainer()
    if (!container) {
      rubberBand = null
      return
    }

    const ids: string[] = []
    container.querySelectorAll('[data-asset-id]').forEach((el) => {
      const r = el.getBoundingClientRect()
      if (
        r.right > rubberBand!.x &&
        r.left < rubberBand!.x + rubberBand!.w &&
        r.bottom > rubberBand!.y &&
        r.top < rubberBand!.y + rubberBand!.h
      ) {
        ids.push((el as HTMLElement).dataset.assetId!)
      }
    })
    if (ids.length > 0) onSelect(ids)
    rubberBand = null
  }

  return {
    get band() {
      return rubberBand
    },
    onMouseDown,
    onMouseMove,
    onMouseUp,
  }
}
