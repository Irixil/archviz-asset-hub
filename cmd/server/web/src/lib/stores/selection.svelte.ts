import type { Asset } from '$lib/api'

let selectedIds = $state(new Set<string>())
let lastSelectedIndex = $state(-1)

export const selectionStore = {
  get selectedIds() {
    return selectedIds
  },
  get lastSelectedIndex() {
    return lastSelectedIndex
  },

  handleCardClick(
    asset: Asset,
    index: number,
    assets: readonly Asset[],
    event: MouseEvent,
    isEditor: boolean
  ): boolean {
    if (!isEditor) return false

    if (
      event.shiftKey &&
      lastSelectedIndex >= 0 &&
      lastSelectedIndex !== index
    ) {
      const next = new Set(selectedIds)
      const lo = Math.min(lastSelectedIndex, index)
      const hi = Math.max(lastSelectedIndex, index)
      for (let i = lo; i <= hi; i++) next.add(assets[i].id)
      selectedIds = next
      return true
    }

    if (event.shiftKey || selectedIds.size > 0) {
      const next = new Set(selectedIds)
      if (next.has(asset.id)) {
        next.delete(asset.id)
      } else {
        next.add(asset.id)
        lastSelectedIndex = index
      }
      selectedIds = next
      return true
    }

    return false
  },

  selectByIds(ids: string[]) {
    const next = new Set(selectedIds)
    for (const id of ids) next.add(id)
    selectedIds = next
  },

  toggle(asset: Asset, index: number) {
    const next = new Set(selectedIds)
    if (next.has(asset.id)) {
      next.delete(asset.id)
      selectedIds = next
      if (next.size === 0) lastSelectedIndex = -1
      return
    }

    next.add(asset.id)
    selectedIds = next
    lastSelectedIndex = index
  },

  remove(id: string) {
    const next = new Set(selectedIds)
    next.delete(id)
    selectedIds = next
    if (selectedIds.size === 0) lastSelectedIndex = -1
  },

  clear() {
    selectedIds = new Set()
    lastSelectedIndex = -1
  },

  selectAll(assets: readonly Asset[]) {
    selectedIds = new Set(assets.map((a) => a.id))
    lastSelectedIndex = assets.length - 1
  },

  invertSelection(assets: readonly Asset[]) {
    const next = new Set<string>()
    for (const a of assets) {
      if (!selectedIds.has(a.id)) next.add(a.id)
    }
    selectedIds = next
    lastSelectedIndex = -1
  },

  moveSelectionTo(index: number, assets: readonly Asset[]) {
    const clamped = Math.max(0, Math.min(assets.length - 1, index))
    selectedIds = new Set([assets[clamped].id])
    lastSelectedIndex = clamped
  },
}
