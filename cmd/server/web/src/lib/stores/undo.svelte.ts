import { ApiError } from '$lib/api/client'
import type { Command } from '$lib/commands/types'
import { m } from '$lib/paraglide/messages'
import { toastStore } from '$lib/stores/toast.svelte'

const MAX_STACK = 50

let undoStack = $state<Command[]>([])
let redoStack = $state<Command[]>([])

export const undoStore = {
  get canUndo() {
    return undoStack.length > 0
  },
  get canRedo() {
    return redoStack.length > 0
  },
  get undoLabel() {
    return undoStack.at(-1)?.label() ?? null
  },
  get redoLabel() {
    return redoStack.at(-1)?.label() ?? null
  },

  async execute(cmd: Command): Promise<void> {
    try {
      await cmd.apply()
      undoStack = [...undoStack.slice(-(MAX_STACK - 1)), cmd]
      redoStack = []
      toastStore.show(cmd.label(), 'success', {
        label: m.undo(),
        onClick: () => undoStore.undo(),
      })
    } catch (err) {
      cmd.rollback?.()
      throw err
    }
  },

  async undo(): Promise<void> {
    const cmd = undoStack.at(-1)
    if (!cmd) return
    try {
      await cmd.revert()
      undoStack = undoStack.slice(0, -1)
      redoStack = [...redoStack, cmd]
      toastStore.show(m.undone({ action: cmd.label() }))
    } catch (e) {
      const msg =
        e instanceof ApiError && e.status === 409
          ? m.undo_conflict()
          : m.undo_failed()
      toastStore.show(msg, 'error')
    }
  },

  async redo(): Promise<void> {
    const cmd = redoStack.at(-1)
    if (!cmd) return
    try {
      await cmd.apply()
      redoStack = redoStack.slice(0, -1)
      undoStack = [...undoStack.slice(-(MAX_STACK - 1)), cmd]
    } catch {
      toastStore.show(m.redo_failed(), 'error')
    }
  },

  clear(): void {
    undoStack = []
    redoStack = []
  },
}
