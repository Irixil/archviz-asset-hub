import type { Asset } from '$lib/api'

export type StackAsset = {
  id: string
  name: string
  thumbnailUrl: string
  mimeType: string
}

export function assetToStack(asset: Asset, thumbUrl: string): StackAsset {
  return {
    id: asset.id,
    name: asset.original_filename,
    thumbnailUrl: thumbUrl,
    mimeType: asset.mime_type,
  }
}

let assets = $state<StackAsset[]>([])
let label = $state<string | null>(null)
let panelOpen = $state(false)

export const stackStore = {
  get assets() {
    return assets
  },
  get label() {
    return label
  },
  get count() {
    return assets.length
  },
  get ids() {
    return assets.map((a) => a.id)
  },
  get panelOpen() {
    return panelOpen
  },
  get active() {
    return assets.length > 0
  },

  inStack(id: string) {
    return assets.some((a) => a.id === id)
  },

  add(asset: StackAsset) {
    if (!assets.some((a) => a.id === asset.id)) {
      assets = [...assets, asset]
    }
  },

  remove(id: string) {
    assets = assets.filter((a) => a.id !== id)
  },

  toggle(asset: StackAsset) {
    if (assets.some((a) => a.id === asset.id)) {
      assets = assets.filter((a) => a.id !== asset.id)
    } else {
      assets = [...assets, asset]
    }
  },

  fromSelection(incoming: StackAsset[]) {
    assets = incoming
    label = null
  },

  setLabel(l: string) {
    label = l || null
  },

  clear() {
    assets = []
    label = null
  },

  openPanel() {
    panelOpen = true
  },
  closePanel() {
    panelOpen = false
  },
  togglePanel() {
    panelOpen = !panelOpen
  },
}
