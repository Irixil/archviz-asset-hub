import type { KeyMap, ShortcutAction } from './types'

export interface Conflict {
  action: ShortcutAction
  combo: string
}

export function findConflicts(
  action: ShortcutAction,
  newCombos: string[],
  keymap: KeyMap
): Conflict[] {
  const conflicts: Conflict[] = []
  for (const [otherAction, otherCombos] of Object.entries(keymap)) {
    if (otherAction === action) continue
    for (const combo of newCombos) {
      if (otherCombos.includes(combo)) {
        conflicts.push({ action: otherAction as ShortcutAction, combo })
      }
    }
  }
  return conflicts
}
