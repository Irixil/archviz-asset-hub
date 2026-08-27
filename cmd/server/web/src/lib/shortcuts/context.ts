import { onDestroy } from 'svelte'
import { onAction, onActions } from './dispatcher'
import type { ShortcutAction } from './types'

export function useShortcut(action: ShortcutAction, handler: () => void): void {
  const unsub = onAction(action, handler)
  onDestroy(unsub)
}

export function useShortcuts(
  map: Partial<Record<ShortcutAction, () => void>>
): void {
  const unsub = onActions(map)
  onDestroy(unsub)
}
