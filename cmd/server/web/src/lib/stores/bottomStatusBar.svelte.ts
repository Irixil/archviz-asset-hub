let slot1 = $state<string | null>(null)
let slot2 = $state<string | null>(null)
let slot3 = $state<string | null>(null)

export type GridMode = 'compact' | 'spaced' | 'table'
type ZoomGridMode = 'compact' | 'spaced'
type ZoomConfig = { default: number; max: number }

const ZOOM_CONFIG: Record<ZoomGridMode, ZoomConfig> = {
  compact: { default: 10, max: 20 },
  spaced: { default: 4, max: 8 },
}

let showZoom = $state(false)

const _storedGridMode = localStorage.getItem(
  'library-grid-mode'
) as GridMode | null
let gridMode = $state<GridMode>(
  _storedGridMode &&
    (['compact', 'spaced', 'table'] as GridMode[]).includes(_storedGridMode)
    ? _storedGridMode
    : 'compact'
)
let zoomByMode = $state<Record<ZoomGridMode, number>>({
  compact: ZOOM_CONFIG.compact.default,
  spaced: ZOOM_CONFIG.spaced.default,
})

const activeZoomMode = $derived<ZoomGridMode>(
  gridMode === 'table' ? 'compact' : gridMode
)
const cfg = $derived(ZOOM_CONFIG[activeZoomMode])
const zoom = $derived(zoomByMode[activeZoomMode])
const sliderMax = $derived(cfg.max - 1)

export const statusBarStore = {
  get slots() {
    return [slot1, slot2, slot3].filter(Boolean)
  },
  get slot1() {
    return slot1
  },
  set slot1(z) {
    slot1 = z
  },
  get slot2() {
    return slot2
  },
  set slot2(z) {
    slot1 = z
  },
  get slot3() {
    return slot3
  },
  set slot3(z) {
    slot1 = z
  },

  get showZoom() {
    return showZoom
  },
  set showZoom(z) {
    showZoom = z
  },

  get gridMode() {
    return gridMode
  },
  set gridMode(m: GridMode) {
    if (gridMode !== 'table' && m !== 'table') {
      const ratio =
        zoomByMode[gridMode as ZoomGridMode] /
        ZOOM_CONFIG[gridMode as ZoomGridMode].max
      zoomByMode[m as ZoomGridMode] = Math.round(
        ratio * ZOOM_CONFIG[m as ZoomGridMode].max
      )
    }
    gridMode = m
    localStorage.setItem('library-grid-mode', m)
  },

  get maxZoom() {
    return cfg.max
  },
  get sliderMax() {
    return sliderMax
  },
  get zoom() {
    return zoom
  },
  set zoom(z) {
    zoomByMode[activeZoomMode] = z
  },
  zoomReset() {
    zoomByMode[activeZoomMode] = cfg.default
  },
  zoomDecrease() {
    zoomByMode[activeZoomMode] = Math.max(0, zoom - 2)
  },
  zoomIncrease() {
    zoomByMode[activeZoomMode] = Math.min(sliderMax, zoom + 2)
  },
}
