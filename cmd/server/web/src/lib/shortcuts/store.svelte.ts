import { browser } from '$app/environment'
import { initDispatcher } from '$lib/shortcuts/dispatcher'
import { DEFAULT_KEYMAP } from '$lib/shortcuts/registry'
import type { KeyMap, ShortcutAction } from '$lib/shortcuts/types'

const STORAGE_KEY = 'damask:keymap'

function loadKeymap(): KeyMap {
  if (!browser) return DEFAULT_KEYMAP
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return DEFAULT_KEYMAP
    const partial = JSON.parse(raw) as Partial<KeyMap>
    return { ...DEFAULT_KEYMAP, ...partial }
  } catch {
    return DEFAULT_KEYMAP
  }
}

const keymapState = $state({ current: loadKeymap() })

$effect.root(() => {
  $effect(() => {
    if (!browser) return
    const overrides: Partial<KeyMap> = {}
    for (const [action, combos] of Object.entries(keymapState.current)) {
      const key = action as ShortcutAction
      if (JSON.stringify(combos) !== JSON.stringify(DEFAULT_KEYMAP[key])) {
        overrides[key] = combos
      }
    }
    if (Object.keys(overrides).length > 0) {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(overrides))
    } else {
      localStorage.removeItem(STORAGE_KEY)
    }
    initDispatcher(keymapState.current)
  })
})

export const keymap = {
  get current(): KeyMap {
    return keymapState.current
  },
}

export function resetShortcut(action: ShortcutAction): void {
  keymapState.current = {
    ...keymapState.current,
    [action]: DEFAULT_KEYMAP[action],
  }
}

export function resetAllShortcuts(): void {
  keymapState.current = { ...DEFAULT_KEYMAP }
}

export function setShortcut(action: ShortcutAction, combos: string[]): void {
  keymapState.current = { ...keymapState.current, [action]: combos }
}
