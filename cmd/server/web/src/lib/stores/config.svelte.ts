import { configApi, type Config } from '$lib/api'

let state = $state<Config>({
  mailHost: '',
  demo: false,
})

export const configStore = {
  get state() {
    return state
  },
  load: () =>
    configApi.load().then((c) => {
      state = c
    }),
}
