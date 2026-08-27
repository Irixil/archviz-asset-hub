export function isMac(): boolean {
  if (typeof navigator === 'undefined') return false
  return /Mac|iPhone|iPad|iPod/.test(navigator.platform)
}

export function modKey(): string {
  return isMac() ? '⌘' : 'Ctrl'
}

export function formatKey(key: string): string {
  return key
    .replace('$mod', modKey())
    .replace('Shift+', '⇧')
    .replace('Alt+', isMac() ? '⌥' : 'Alt+')
    .replace('ArrowLeft', '←')
    .replace('ArrowRight', '→')
    .replace('ArrowUp', '↑')
    .replace('ArrowDown', '↓')
    .replace('Escape', 'Esc')
    .replace('Delete', '⌫')
    .replace('Backspace', '⌫')
}
