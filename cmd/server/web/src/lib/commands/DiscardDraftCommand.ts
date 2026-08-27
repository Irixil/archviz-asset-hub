import { m } from '$lib/paraglide/messages'
import type { Command } from './types'

export class DiscardDraftCommand implements Command {
  constructor(
    private onApply: () => void,
    private onRevert: () => void
  ) {}

  label() {
    return m.variants_draft_discard()
  }

  apply(): Promise<void> {
    this.onApply()
    return Promise.resolve()
  }

  revert(): Promise<void> {
    this.onRevert()
    return Promise.resolve()
  }
}
