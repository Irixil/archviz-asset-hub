import { tagApi } from '$lib/api'
import { m } from '$lib/paraglide/messages'
import { assetsStore } from '$lib/stores/assets.svelte'
import { tagsManagementStore } from '$lib/stores/tagsManagement.svelte'
import type { Command } from './types'

export class TagAsset implements Command {
  constructor(
    private assetId: string,
    private tag: string,
    private direction: 'add' | 'remove'
  ) {}

  label() {
    return this.direction === 'add'
      ? m.cmd_tag_add({ tag: this.tag })
      : m.cmd_tag_remove({ tag: this.tag })
  }

  async apply() {
    if (this.direction === 'add') {
      await tagApi.addToAsset(this.assetId, this.tag)
      assetsStore.addTag(this.assetId, this.tag)
    } else {
      await tagApi.removeFromAsset(this.assetId, this.tag)
      assetsStore.removeTag(this.assetId, this.tag)
    }
    tagsManagementStore.invalidate()
  }

  async revert() {
    const inverse = new TagAsset(
      this.assetId,
      this.tag,
      this.direction === 'add' ? 'remove' : 'add'
    )
    await inverse.apply()
  }
}
