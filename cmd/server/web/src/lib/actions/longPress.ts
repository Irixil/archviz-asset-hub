interface LongPressOptions {
  duration?: number
  onLongPress: () => void
  onTap?: () => void
}

export function longPress(node: HTMLElement, options: LongPressOptions) {
  let opts = options
  let timer: ReturnType<typeof setTimeout> | null = null
  let startX = 0
  let startY = 0
  let longPressed = false
  let suppressClick = false

  function clearTimer() {
    if (timer) {
      clearTimeout(timer)
      timer = null
    }
  }

  function cancel() {
    clearTimer()
    longPressed = false
  }

  function onPointerDown(event: PointerEvent) {
    if (
      event.pointerType !== 'touch' &&
      event.pointerType !== 'pen' &&
      event.button !== 0
    ) {
      return
    }

    startX = event.clientX
    startY = event.clientY
    longPressed = false
    suppressClick = false
    clearTimer()

    timer = setTimeout(() => {
      longPressed = true
      suppressClick = true
      if ('vibrate' in navigator) navigator.vibrate(10)
      opts.onLongPress()
    }, opts.duration ?? 500)
  }

  function onPointerMove(event: PointerEvent) {
    const dx = Math.abs(event.clientX - startX)
    const dy = Math.abs(event.clientY - startY)
    if (dx > 10 || dy > 10) clearTimer()
  }

  function onPointerUp() {
    clearTimer()
    if (!longPressed) opts.onTap?.()
    longPressed = false
  }

  function onClick(event: MouseEvent) {
    if (!suppressClick) return
    event.preventDefault()
    event.stopImmediatePropagation()
    suppressClick = false
  }

  node.addEventListener('pointerdown', onPointerDown)
  node.addEventListener('pointermove', onPointerMove)
  node.addEventListener('pointerup', onPointerUp)
  node.addEventListener('pointercancel', cancel)
  node.addEventListener('pointerleave', clearTimer)
  node.addEventListener('click', onClick, true)

  return {
    update(nextOptions: LongPressOptions) {
      opts = nextOptions
    },
    destroy() {
      cancel()
      node.removeEventListener('pointerdown', onPointerDown)
      node.removeEventListener('pointermove', onPointerMove)
      node.removeEventListener('pointerup', onPointerUp)
      node.removeEventListener('pointercancel', cancel)
      node.removeEventListener('pointerleave', clearTimer)
      node.removeEventListener('click', onClick, true)
    },
  }
}
