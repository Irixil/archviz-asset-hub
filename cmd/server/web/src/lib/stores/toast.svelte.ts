type ToastAction = { label: string; onClick: () => void }

let toast = $state<{
  msg: string
  type: 'success' | 'error'
  action?: ToastAction
} | null>(null)
let timer: ReturnType<typeof setTimeout>

export const toastStore = {
  get current() {
    return toast
  },

  show(
    msg: string,
    type: 'success' | 'error' = 'success',
    action?: ToastAction
  ) {
    clearTimeout(timer)
    toast = { msg, type, action }
    timer = setTimeout(
      () => {
        toast = null
      },
      action ? 7000 : 3000
    )
  },

  dismiss() {
    clearTimeout(timer)
    toast = null
  },
}
