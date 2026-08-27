// store whether we're on a narrow device
let smallDevice = $state(false)
let isTouch = $state(false)
let listening = false

export const onSmallDevice = () => smallDevice

export const browserDetectStore = {
  get onSmallDevice() {
    return smallDevice
  },
  get onWideDevice() {
    return !smallDevice
  },
  get isTouch() {
    return isTouch
  },

  detectSmallDevice() {
    if (typeof window === 'undefined' || listening) return
    listening = true

    // attach a media query listener to the window
    const mediaQuery = window.matchMedia('(width <= 640px)')
    const touchQuery = window.matchMedia('(pointer: coarse)')

    smallDevice = mediaQuery.matches
    isTouch = touchQuery.matches

    // every time the media query matches or unmatches
    mediaQuery.addEventListener('change', ({ matches }) => {
      // set the state of our variable
      smallDevice = matches
    })

    touchQuery.addEventListener('change', ({ matches }) => {
      isTouch = matches
    })
  },
}
