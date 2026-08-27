import type { Asset } from '$lib/api'
import { SvelteSet } from 'svelte/reactivity'

export interface UploadItem {
  id: string
  file: File
  progress: number
  status: 'uploading' | 'processing' | 'done' | 'error'
  asset?: Asset
  error?: string
}

let timer: number | null = null
let items = $state<UploadItem[]>([])
// IDs of assets completed in this session — used for the required-fields nudge
let recentlyUploadedIds = $state(new SvelteSet<string>())

export const uploadsStore = {
  get items() {
    return items
  },

  get recentlyUploadedIds() {
    return recentlyUploadedIds
  },

  add(item: UploadItem) {
    if (timer) clearTimeout(timer)
    items = [...items, item]
  },

  update(id: string, patch: Partial<UploadItem>) {
    items = items.map((item) => (item.id === id ? { ...item, ...patch } : item))
    // Track newly completed uploads by their asset ID
    if (patch.status === 'done' && patch.asset) {
      recentlyUploadedIds = new SvelteSet([
        ...recentlyUploadedIds,
        patch.asset.id,
      ])
    }
    if (items.every((i) => i.status === 'done')) {
      timer = +setTimeout(() => uploadsStore.clear(), 10_000)
    }
  },

  remove(id: string) {
    items = items.filter((item) => item.id !== id)
  },

  clear() {
    items = items.filter(
      (item) => item.status === 'uploading' || item.status === 'processing'
    )
  },

  clearRecentlyUploaded() {
    recentlyUploadedIds = new SvelteSet()
  },
}
