import { tinykeys } from 'tinykeys'
import { ACTION_CONTEXT, DEFAULT_KEYMAP } from './registry'
import { activateGMode } from './sequence'
import type { KeyMap, ShortcutAction } from './types'

const ACTION_EVENT = 'damask:shortcut'

let activeContext = 'grid'

export function setShortcutContext(ctx: string): void {
  activeContext = ctx
}

function shouldIgnore(e: KeyboardEvent): boolean {
  const target = e.target as HTMLElement
  if (!target) return false
  if (target.isContentEditable) return true
  return ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}

function guarded(fn: (e: KeyboardEvent) => void) {
  return (e: KeyboardEvent) => {
    if (shouldIgnore(e)) return
    fn(e)
  }
}

function dispatch(action: ShortcutAction): void {
  document.dispatchEvent(
    new CustomEvent<{ action: ShortcutAction }>(ACTION_EVENT, {
      detail: { action },
      bubbles: false,
    })
  )
}

let unbind: (() => void) | null = null

export function initDispatcher(keymap: KeyMap = DEFAULT_KEYMAP): void {
  if (unbind) unbind()

  const bindings: Record<string, (e: KeyboardEvent) => void> = {}

  // g-mode visual indicator: fire when bare 'g' is pressed (sequence preamble)
  bindings['g'] = guarded(() => activateGMode())

  const comboActions: Record<string, ShortcutAction[]> = {}
  for (const [actionStr, combos] of Object.entries(keymap)) {
    const action = actionStr as ShortcutAction
    for (const combo of combos) {
      ;(comboActions[combo] ??= []).push(action)
    }
  }
  for (const [combo, actions] of Object.entries(comboActions)) {
    bindings[combo] = guarded((e) => {
      e.preventDefault()
      const specific = actions.find(
        (a) => ACTION_CONTEXT[a]?.context === activeContext
      )
      const global = actions.find((a) => !ACTION_CONTEXT[a]?.context)
      const action = specific ?? global
      if (action) dispatch(action)
    })
  }

  unbind = tinykeys(window, bindings)
}

export function onAction(
  action: ShortcutAction,
  handler: () => void
): () => void {
  function listener(e: Event) {
    const custom = e as CustomEvent<{ action: ShortcutAction }>
    if (custom.detail.action === action) handler()
  }
  document.addEventListener(ACTION_EVENT, listener)
  return () => document.removeEventListener(ACTION_EVENT, listener)
}

export function triggerAction(action: ShortcutAction): void {
  const meta = ACTION_CONTEXT[action]
  if (meta?.context && meta.context !== activeContext) return
  dispatch(action)
}

export function onActions(
  map: Partial<Record<ShortcutAction, () => void>>
): () => void {
  const unsubs = Object.entries(map).map(([action, handler]) =>
    onAction(action as ShortcutAction, handler!)
  )
  return () => unsubs.forEach((u) => u())
}
