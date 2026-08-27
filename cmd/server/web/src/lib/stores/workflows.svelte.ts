import { workflowsApi, type Workflow } from '$lib/api/workflows'
import { toastStore } from './toast.svelte'

let workflows = $state<Workflow[]>([])
let loading = $state(false)
let loaded = $state(false)
let selectedId = $state<string | null>(null)

export const workflowsStore = {
  get workflows() {
    return workflows
  },
  get loading() {
    return loading
  },
  get loaded() {
    return loaded
  },
  get selectedId() {
    return selectedId
  },
  set selectedId(id: string | null) {
    selectedId = id
  },

  async load() {
    if (loading) return
    loading = true
    try {
      workflows = await workflowsApi.list()
      loaded = true
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to load workflows.',
        'error'
      )
    } finally {
      loading = false
    }
  },

  upsert(workflow: Workflow) {
    const idx = workflows.findIndex((w) => w.id === workflow.id)
    if (idx >= 0) {
      workflows = workflows.map((w) => (w.id === workflow.id ? workflow : w))
    } else {
      workflows = [workflow, ...workflows]
    }
  },

  patch(id: string, patch: Partial<Workflow>) {
    workflows = workflows.map((w) => (w.id === id ? { ...w, ...patch } : w))
  },

  remove(id: string) {
    workflows = workflows.filter((w) => w.id !== id)
  },
}
