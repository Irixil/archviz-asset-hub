export type ShortcutAction =
  | 'palette.open'
  | 'search.focus'
  | 'upload.open'
  | 'asset.delete'
  | 'asset.download'
  | 'asset.rename'
  | 'asset.share'
  | 'asset.open-detail'
  | 'selection.all'
  | 'selection.clear'
  | 'selection.invert'
  | 'view.toggle-layout'
  | 'view.zoom-in'
  | 'view.zoom-out'
  | 'view.zoom-reset'
  | 'lightbox.close'
  | 'lightbox.next'
  | 'lightbox.prev'
  | 'lightbox.download'
  | 'lightbox.zoom-in'
  | 'lightbox.zoom-out'
  | 'navigate.library'
  | 'navigate.tags'
  | 'navigate.settings'
  | 'navigate.shares'
  | 'help.toggle'
  | 'sidebar.toggle'
  | 'selection.move-right'
  | 'selection.move-left'
  | 'selection.move-up'
  | 'selection.move-down'
  | 'history.undo'
  | 'history.redo'

export type KeyMap = Record<ShortcutAction, string[]>

export interface ActionMeta {
  context?: string
}

export type ActionMetaMap = Partial<Record<ShortcutAction, ActionMeta>>

export type ShortcutHandler = (action: ShortcutAction) => boolean | void

export interface HandlerRegistration {
  action: ShortcutAction
  handler: ShortcutHandler
  label: string
}

export interface ShortcutGroup {
  title: string
  actions: Array<{
    action: ShortcutAction
    label: string
    contextual?: boolean
  }>
}
