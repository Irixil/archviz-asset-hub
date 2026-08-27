import { collectionApi, type Collection } from '$lib/api'

let collections = $state<Collection[]>([])
let loaded = $state(false)

export const collectionsStore = {
  get collections() {
    return collections
  },
  get loaded() {
    return loaded
  },

  async load() {
    try {
      collections = await collectionApi.list()
      loaded = true
    } catch {
      // ignore
    }
  },

  async rename(id: string, name: string) {
    const col = collections.find((c) => c.id === id)
    if (!col) return
    const updated = await collectionApi.update(id, name, col.description)
    collections = collections.map((c) => (c.id === id ? updated : c))
  },

  async remove(id: string) {
    await collectionApi.delete(id)
    collections = collections.filter((c) => c.id !== id)
  },

  invalidate() {
    loaded = false
    this.load()
  },
}
