import { assetApi } from '$lib/api'
import { m } from '$lib/paraglide/messages'
import { assetsStore } from '$lib/stores/assets.svelte'
import type { Command } from './types'

function stemOf(filename: string) {
  const dot = filename.lastIndexOf('.')
  return dot > 0 ? filename.slice(0, dot) : filename
}

export class RenameAsset implements Command {
  private beforeStem: string
  private afterStem: string

  constructor(
    private assetId: string,
    before: string,
    after: string
  ) {
    this.beforeStem = stemOf(before)
    this.afterStem = after
  }

  label() {
    return m.cmd_rename_asset({ name: this.afterStem })
  }

  async apply() {
    const updated = await assetApi.rename(this.assetId, this.afterStem)
    assetsStore.patchAsset(this.assetId, {
      original_filename: updated.original_filename,
    })
  }

  async revert() {
    const updated = await assetApi.rename(this.assetId, this.beforeStem)
    assetsStore.patchAsset(this.assetId, {
      original_filename: updated.original_filename,
    })
  }
}
