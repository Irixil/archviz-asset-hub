<script lang="ts">
  import { fieldDefinitionApi } from '$lib/api'
  import type { FieldDefinition, FieldFilter } from '$lib/api'
  import { onMount } from 'svelte'
  import Chip from '$lib/components/ui/Chip.svelte'
  import FieldFilterInput from '$lib/components/FieldFilterInput.svelte'
  import { m } from '$lib/paraglide/messages'
  import { ChevronDown, SlidersHorizontal } from '@lucide/svelte'

  interface Props {
    activeFilters: FieldFilter[]
    onchange: (filters: FieldFilter[]) => void
  }

  let { activeFilters, onchange }: Props = $props()

  let definitions = $state<FieldDefinition[]>([])
  let showExif = $state(false)
  let expanded = $state(false)
  let loadError = $state(false)

  async function loadDefinitions() {
    loadError = false
    try {
      definitions = await fieldDefinitionApi.list('asset')
    } catch {
      loadError = true
    }
  }

  onMount(loadDefinitions)

  // Per-field input state keyed by field.key.
  // text/url: string, number: { min, max }, date: { from, to }, boolean: '' | 'true' | 'false', select: string[]
  type LocalState = Record<string, unknown>
  let local = $state<LocalState>({})

  $effect(() => {
    // When definitions load, seed any new keys without touching existing ones (preserves user input).
    for (const def of definitions) {
      if (def.key in local) continue
      switch (def.field_type) {
        case 'number':
          local[def.key] = { min: '', max: '' }
          break
        case 'date':
          local[def.key] = { from: '', to: '' }
          break
        default:
          local[def.key] = ''
          break
      }
    }
  })

  // Rebuild filters from local state and emit
  function emit() {
    const filters: FieldFilter[] = []
    for (const def of definitions) {
      const v = local[def.key]
      if (v === undefined || v === null) continue
      switch (def.field_type) {
        case 'text':
        case 'url': {
          const s = (v as string).trim()
          if (s) filters.push({ key: def.key, op: 'contains', value: s })
          break
        }
        case 'number': {
          const { min, max } = v as { min: string; max: string }
          if (min.trim())
            filters.push({ key: def.key, op: 'gte', value: min.trim() })
          if (max.trim())
            filters.push({ key: def.key, op: 'lte', value: max.trim() })
          break
        }
        case 'date': {
          const { from, to } = v as { from: string; to: string }
          if (from) filters.push({ key: def.key, op: 'gte', value: from })
          if (to) filters.push({ key: def.key, op: 'lte', value: to })
          break
        }
        case 'boolean': {
          const s = v as string
          if (s === 'true')
            filters.push({ key: def.key, op: 'eq', value: 'true' })
          else if (s === 'false')
            filters.push({ key: def.key, op: 'eq', value: 'false' })
          break
        }
        case 'select': {
          const s = v as string
          if (s) filters.push({ key: def.key, op: 'eq', value: s })
          break
        }
      }
    }
    onchange(filters)
  }

  let debounceTimer: ReturnType<typeof setTimeout>
  function debouncedEmit() {
    clearTimeout(debounceTimer)
    debounceTimer = setTimeout(emit, 350)
  }

  function clearField(key: string) {
    const def = definitions.find((d) => d.key === key)
    if (!def) return
    switch (def.field_type) {
      case 'text':
      case 'url':
        local[key] = ''
        break
      case 'number':
        local[key] = { min: '', max: '' }
        break
      case 'date':
        local[key] = { from: '', to: '' }
        break
      case 'boolean':
        local[key] = ''
        break
      case 'select':
        local[key] = ''
        break
    }
    emit()
  }

  // Chip label for an active filter
  function chipLabel(f: FieldFilter): string {
    const def = definitions.find((d) => d.key === f.key)
    const name = def?.name ?? f.key
    const opLabels: Record<string, string> = {
      eq: '=',
      lt: '<',
      lte: '≤',
      gt: '>',
      gte: '≥',
      contains: '~',
      starts_with: '^',
    }
    return `${name} ${opLabels[f.op] ?? f.op} ${f.value}`
  }

  function removeChip(f: FieldFilter) {
    clearField(f.key)
  }

  const activeDefinitions = $derived(
    definitions.filter((d) => !d.deleted_at && !d.key.startsWith('_exif_'))
  )
  const exifDefinitions = $derived(
    definitions.filter((d) => !d.deleted_at && d.key.startsWith('_exif_'))
  )
  const hasFilters = $derived(activeFilters.length > 0)
</script>

{#if activeDefinitions.length > 0 || exifDefinitions.length > 0 || loadError}
  <div class="border-t border-[var(--border-subtle)] bg-[var(--bg-surface)]">
    <div class="flex min-h-11 flex-wrap items-center gap-2 px-3 py-1.5 sm:px-4">
      <button
        type="button"
        class="flex min-h-8 items-center gap-1.5 rounded-md border border-[var(--border-default)] bg-[var(--bg-app)] px-2.5 text-xs font-semibold text-[var(--text-secondary)] hover:border-[var(--accent)] hover:text-[var(--text-primary)]"
        aria-expanded={expanded}
        onclick={() => (expanded = !expanded)}
      >
        <SlidersHorizontal class="h-3.5 w-3.5" />
        专业属性{hasFilters ? ` ${activeFilters.length}` : ''}
        <ChevronDown
          class="h-3.5 w-3.5 transition-transform {expanded
            ? 'rotate-180'
            : ''}"
        />
      </button>

      {#if loadError}
        <span class="text-xs text-[var(--text-muted)]"
          >专业属性暂时无法加载</span
        >
        <button
          type="button"
          class="text-xs font-semibold text-[var(--accent-text)] hover:underline"
          onclick={loadDefinitions}>重试</button
        >
      {/if}

      <!-- Active filter chips -->
      {#if hasFilters}
        <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
          {#each activeFilters as f}
            <Chip
              label={chipLabel(f)}
              onremove={() => removeChip(f)}
              color="#a96618"
            />
          {/each}
          <button
            type="button"
            class="ml-1 min-h-8 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)]"
            onclick={() => {
              for (const def of definitions) clearField(def.key)
            }}
          >
            {m.clear_all()}
          </button>
        </div>
      {/if}
    </div>

    <!-- Filter controls per field -->
    {#if expanded && activeDefinitions.length > 0}
      <div
        class="flex flex-wrap items-end gap-x-6 gap-y-3 border-t border-[var(--border-subtle)] bg-[var(--bg-stripe)] px-4 py-4"
      >
        {#each activeDefinitions as def}
          <FieldFilterInput
            {def}
            {local}
            onchange={emit}
            ondebouncedchange={debouncedEmit}
          />
        {/each}

        <!-- EXIF fields (collapsed by default) -->
        {#if exifDefinitions.length > 0}
          {#if !showExif}
            <button
              class="flex min-h-9 items-center gap-1 rounded-md px-2 text-sm text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
              onclick={() => {
                showExif = !showExif
              }}
            >
              {m.exif_fields()}
            </button>
          {:else}
            {#each exifDefinitions as def}
              <FieldFilterInput
                {def}
                {local}
                onchange={emit}
                ondebouncedchange={debouncedEmit}
              />
            {/each}
          {/if}
        {/if}
      </div>
    {/if}
  </div>
{/if}
