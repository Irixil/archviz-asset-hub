import { browser } from '$app/environment'

const STORAGE_KEY = 'theme'

type ThemeMode = 'light' | 'dark'

let dark = $state(false)
let initialized = false
let systemThemeQuery: MediaQueryList | null = null
let systemThemeListener: ((event: MediaQueryListEvent) => void) | null = null

function getStoredTheme(): ThemeMode | null {
  if (!browser) return null

  const stored = localStorage.getItem(STORAGE_KEY)
  return stored === 'dark' || stored === 'light' ? stored : null
}

function getPreferredDark() {
  if (!browser) return false
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function applyTheme(isDark: boolean) {
  if (!browser) return
  document.documentElement.classList.toggle('dark', isDark)
}

function syncTheme() {
  const stored = getStoredTheme()
  dark = stored === 'dark' || (!stored && getPreferredDark())
  applyTheme(dark)
}

function handleSystemThemeChange(event: MediaQueryListEvent) {
  if (getStoredTheme()) return
  dark = event.matches
  applyTheme(dark)
}

export const themeStore = {
  get dark() {
    return dark
  },

  init() {
    if (!browser || initialized) return

    initialized = true
    syncTheme()

    systemThemeQuery = window.matchMedia('(prefers-color-scheme: dark)')
    systemThemeListener = handleSystemThemeChange
    systemThemeQuery.addEventListener('change', systemThemeListener)
  },

  toggle() {
    const nextDark = !dark
    dark = nextDark
    localStorage.setItem(STORAGE_KEY, nextDark ? 'dark' : 'light')
    applyTheme(nextDark)
  },
}
