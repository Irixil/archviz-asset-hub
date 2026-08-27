import { collectionApi } from '$lib/api'
import { m } from '$lib/paraglide/messages'
import { assetsStore } from '$lib/stores/assets.svelte'
import { collectionsStore } from '$lib/stores/collections.svelte'
import type { Command } from './types'

export class RemoveAssetFromCollection implements Command {
  constructor(
    private assetId: string,
    private collection: string,
    private collectionName: string
  ) {}

  label() {
    return m.cmd_unassign_collection({ name: this.collectionName ?? '—' })
  }

  async apply() {
    await collectionApi.removeAsset(this.collection, this.assetId)
    collectionsStore.invalidate()
    assetsStore.invalidate()
  }

  async revert() {
    await collectionApi.addAsset(this.collection, this.assetId)
    collectionsStore.invalidate()
    assetsStore.invalidate()
  }
}
