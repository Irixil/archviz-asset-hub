<script lang="ts">
  import type { WorkflowNodeSchema } from '$lib/api/workflows'

  interface Props {
    schemas: WorkflowNodeSchema[]
    onAdd?: (nodeType: string) => void
    disabled?: boolean
    hasTrigger?: boolean
  }

  let {
    schemas,
    onAdd = () => {},
    disabled = false,
    hasTrigger = false,
  }: Props = $props()

  function isNodeDisabled(category: string): boolean {
    if (disabled) return true
    if (category === 'trigger') return hasTrigger
    return !hasTrigger
  }

  const groupNames = ['trigger', 'filter', 'action']
  const grouped = $derived.by(() => {
    const groups = new Map<string, WorkflowNodeSchema[]>()
    for (const schema of schemas) {
      const list = groups.get(schema.category) ?? []
      list.push(schema)
      groups.set(schema.category, list)
    }
    return Array.from(groups.entries()).sort((a, b) => {
      const ai = groupNames.indexOf(a[0])
      const bi = groupNames.indexOf(b[0])
      if (ai === -1 && bi === -1) return a[0].localeCompare(b[0])
      if (ai === -1) return 1
      if (bi === -1) return -1
      return ai - bi
    })
  })

  function accentDot(category: string) {
    switch (category) {
      case 'trigger':
        return 'bg-sky-400'
      case 'filter':
        return 'bg-amber-400'
      case 'action':
        return 'bg-emerald-400'
      default:
        return 'bg-slate-400'
    }
  }

  function accentCard(category: string) {
    switch (category) {
      case 'trigger':
        return 'border-sky-500/25 bg-sky-500/5 hover:bg-sky-500/10 hover:border-sky-500/40'
      case 'filter':
        return 'border-amber-500/25 bg-amber-500/5 hover:bg-amber-500/10 hover:border-amber-500/40'
      case 'action':
        return 'border-emerald-500/25 bg-emerald-500/5 hover:bg-emerald-500/10 hover:border-emerald-500/40'
      default:
        return 'border-[var(--border-subtle)] bg-[var(--bg-elevated)] hover:bg-[var(--bg-hover)]'
    }
  }

  function accentText(category: string) {
    switch (category) {
      case 'trigger':
        return 'text-sky-700 dark:text-sky-300'
      case 'filter':
        return 'text-amber-700 dark:text-amber-300'
      case 'action':
        return 'text-emerald-700 dark:text-emerald-300'
      default:
        return 'text-[var(--text-primary)]'
    }
  }
</script>

<div class="flex flex-col gap-4 p-4">
  <div class="mt-1.5 text-xs text-[var(--text-secondary)]">
    Click to add nodes.
  </div>

  {#each grouped as [category, items] (category)}
    <div class="space-y-1.5">
      <div class="flex items-center gap-2 px-0.5">
        <span class="h-1.5 w-1.5 rounded-full {accentDot(category)}"></span>
        <p
          class="text-[10px] font-semibold tracking-[0.16em] text-[var(--text-muted)] uppercase"
        >
          {category}s
        </p>
      </div>
      <div class="grid gap-1">
        {#each items as schema (schema.type)}
          <button
            type="button"
            data-node-type={schema.type}
            class="group rounded-xl border px-3 py-2 text-left transition-all disabled:cursor-not-allowed disabled:opacity-50 {accentCard(
              category
            )}"
            disabled={isNodeDisabled(category)}
            onclick={() => onAdd(schema.type)}
          >
            <span class="block text-[13px] font-semibold {accentText(category)}"
              >{schema.label}</span
            >
          </button>
        {/each}
      </div>
    </div>
  {/each}
</div>
