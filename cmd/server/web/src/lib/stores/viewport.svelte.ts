import { browser } from '$app/environment'

export const BREAKPOINTS = {
  sm: 640,
  md: 768,
  lg: 1024,
  xl: 1280,
} as const

export type Breakpoint = 'xs' | 'sm' | 'md' | 'lg' | 'xl'

function getBreakpoint(width: number): Breakpoint {
  if (width < BREAKPOINTS.sm) return 'xs'
  if (width < BREAKPOINTS.md) return 'sm'
  if (width < BREAKPOINTS.lg) return 'md'
  if (width < BREAKPOINTS.xl) return 'lg'
  return 'xl'
}

let viewportWidth = $state<number>(browser ? window.innerWidth : 1280)
let isTouch = $state(false)

function syncViewport(width: number) {
  viewportWidth = width
}

if (browser) {
  isTouch = window.matchMedia('(pointer: coarse)').matches

  const touchQuery = window.matchMedia('(pointer: coarse)')
  const handleTouchChange = (event: MediaQueryListEvent) => {
    isTouch = event.matches
  }
  touchQuery.addEventListener('change', handleTouchChange)

  const observer = new ResizeObserver((entries) => {
    const entry = entries[0]
    if (!entry) return
    syncViewport(entry.contentRect.width)
  })
  observer.observe(document.documentElement)
}

export const viewportStore = {
  get width() {
    return viewportWidth
  },
  get breakpoint(): Breakpoint {
    return getBreakpoint(viewportWidth)
  },
  get isMobile() {
    return viewportWidth < BREAKPOINTS.md
  },
  get isXl() {
    return viewportWidth >= BREAKPOINTS.xl
  },
  get isTouch() {
    return isTouch
  },
}
