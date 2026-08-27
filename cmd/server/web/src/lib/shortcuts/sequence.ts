import { writable } from 'svelte/store'

export const gModeActive = writable(false)

let gModeTimer: ReturnType<typeof setTimeout> | null = null

export function activateGMode(): void {
  gModeActive.set(true)
  if (gModeTimer) clearTimeout(gModeTimer)
  gModeTimer = setTimeout(() => gModeActive.set(false), 800)
}

export function clearGMode(): void {
  gModeActive.set(false)
  if (gModeTimer) clearTimeout(gModeTimer)
}
