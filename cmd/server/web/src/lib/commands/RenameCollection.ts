import { m } from '$lib/paraglide/messages'
import { collectionsStore } from '$lib/stores/collections.svelte'
import type { Command } from './types'

export class RenameCollection implements Command {
  constructor(
    private id: string,
    private before: string,
    private after: string
  ) {}

  label() {
    return m.cmd_rename_collection({ name: this.after })
  }

  async apply() {
    await collectionsStore.rename(this.id, this.after)
  }

  async revert() {
    await collectionsStore.rename(this.id, this.before)
  }
}
