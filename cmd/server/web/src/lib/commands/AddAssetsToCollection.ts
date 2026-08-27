import { collectionApi } from '$lib/api'
import { m } from '$lib/paraglide/messages'
import { assetsStore } from '$lib/stores/assets.svelte'
import { collectionsStore } from '$lib/stores/collections.svelte'
import type { Command } from './types'

export class AddAssetsToCollection implements Command {
  constructor(
    private assetIds: string[],
    private collection: string,
    private collectionName: string
  ) {}

  label() {
    return m.cmd_assign_collection({ name: this.collectionName ?? '—' })
  }

  async apply() {
    await Promise.all(
      this.assetIds.map((id) => collectionApi.addAsset(this.collection, id))
    )
    collectionsStore.invalidate()
    assetsStore.invalidate()
  }

  async revert() {
    await Promise.all(
      this.assetIds.map((id) => collectionApi.removeAsset(this.collection, id))
    )
    collectionsStore.invalidate()
    assetsStore.invalidate()
  }
}
