import { tagApi } from '$lib/api'
import { m } from '$lib/paraglide/messages'
import { assetsStore } from '$lib/stores/assets.svelte'
import { tagsManagementStore } from '$lib/stores/tagsManagement.svelte'
import type { Command } from './types'

export class BulkTagAsset implements Command {
  constructor(
    private assetIds: string[],
    private tag: string
  ) {}

  label() {
    return m.cmd_bulk_tag({
      count: String(this.assetIds.length),
      tag: this.tag,
    })
  }

  async apply() {
    await tagApi.bulkTag(this.assetIds, this.tag)
    for (const id of this.assetIds) assetsStore.addTag(id, this.tag)
    tagsManagementStore.invalidate()
  }

  async revert() {
    const results = await Promise.allSettled(
      this.assetIds.map((id) => tagApi.removeFromAsset(id, this.tag))
    )
    for (let i = 0; i < this.assetIds.length; i++) {
      if (results[i].status === 'fulfilled')
        assetsStore.removeTag(this.assetIds[i], this.tag)
    }
    tagsManagementStore.invalidate()
    if (results.some((r) => r.status === 'rejected')) {
      throw new Error('partial revert failure')
    }
  }

  rollback() {
    for (const id of this.assetIds) assetsStore.removeTag(id, this.tag)
  }
}
