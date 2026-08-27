<script lang="ts">
  import { fieldDefinitionApi, projectFieldApi } from '$lib/api'
  import type { FieldDefinition, ProjectFieldValue } from '$lib/api'
  import { ChevronDown, ChevronRight } from '@lucide/svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import SubSectionTitle from './ui/SubSectionTitle.svelte'
  import FieldCard from './FieldCard.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    projectId: string
  }

  let { projectId }: Props = $props()

  let definitions = $state<readonly FieldDefinition[]>([])
  let values = $state<readonly ProjectFieldValue[]>([])
  let loading = $state(true)
  let showDeprecated = $state(false)

  let editingFieldId = $state<string | null>(null)
  let editValue = $state<string>('')
  let savingFieldId = $state<string | null>(null)
  let saveSuccess = $state<string | null>(null)
  let saveError = $state<string | null>(null)

  $effect(() => {
    if (projectId) load()
  })

  async function load() {
    loading = true
    try {
      const [defs, vals] = await Promise.all([
        fieldDefinitionApi.list('project'),
        projectFieldApi.get(projectId),
      ])
      definitions = defs
      values = vals.fields
    } finally {
      loading = false
    }
  }

  function valueFor(fieldId: string): ProjectFieldValue | undefined {
    return values.find((v) => v.field_id === fieldId)
  }

  function displayValue(fv: ProjectFieldValue): string {
    if (fv.value === null || fv.value === undefined) return ''
    if (fv.field_type === 'boolean') return fv.value ? m.yes() : m.no()
    return String(fv.value)
  }

  function startEdit(def: FieldDefinition) {
    const fv = valueFor(def.id)
    editingFieldId = def.id
    saveError = null
    saveSuccess = null
    if (fv && fv.value !== null && fv.value !== undefined) {
      editValue =
        def.field_type === 'boolean'
          ? fv.value
            ? 'true'
            : 'false'
          : String(fv.value)
    } else {
      editValue = def.field_type === 'boolean' ? 'false' : ''
    }
  }

  async function saveField(def: FieldDefinition) {
    savingFieldId = def.id
    saveError = null
    saveSuccess = null
    try {
      let parsedValue: string | number | boolean | null = null

      if (editValue === '' || editValue === null) {
        parsedValue = null
      } else if (def.field_type === 'number') {
        const n = parseFloat(editValue)
        if (isNaN(n)) {
          saveError = m.digit_required()
          savingFieldId = null
          return
        }
        parsedValue = n
      } else if (def.field_type === 'boolean') {
        parsedValue = editValue === 'true'
      } else {
        parsedValue = editValue
      }

      const result = await projectFieldApi.patch(projectId, [
        { field_id: def.id, value: parsedValue },
      ])
      values = result.fields
      editingFieldId = null
      saveSuccess = def.id
      setTimeout(() => {
        if (saveSuccess === def.id) saveSuccess = null
      }, 2000)
    } catch (e: unknown) {
      saveError = e instanceof Error ? e.message : m.save_failed()
    } finally {
      savingFieldId = null
    }
  }

  function cancelEdit() {
    editingFieldId = null
    saveError = null
  }

  function handleKeydown(e: KeyboardEvent, def: FieldDefinition) {
    if (e.key === 'Enter' && def.field_type !== 'text') {
      e.preventDefault()
      saveField(def)
    }
    if (e.key === 'Escape') cancelEdit()
  }

  function toggleBoolean(def: FieldDefinition) {
    const fv = valueFor(def.id)
    editValue = fv?.value ? 'false' : 'true'
    editingFieldId = def.id
    saveField(def)
  }

  const activeDefinitions = $derived(
    definitions.filter((d) => !d.deleted_at && !d.key.startsWith('_exif_'))
  )
  const orphanedValues = $derived(values.filter((v) => v.definition_deleted))
</script>

<div>
  <SubSectionTitle>项目字段</SubSectionTitle>

  {#if loading}
    <div class="flex justify-center py-4">
      <Spinner size="sm" />
    </div>
  {:else if activeDefinitions.length === 0}
    <p class="text-sm text-gray-400 dark:text-gray-500">
      {m.no_project_fields_yet()}
    </p>
  {:else}
    <div class="space-y-2">
      {#each activeDefinitions as def (def.id)}
        <FieldCard
          {def}
          fv={valueFor(def.id)}
          isEditing={editingFieldId === def.id}
          isSaving={savingFieldId === def.id}
          didSave={saveSuccess === def.id}
          {editValue}
          {saveError}
          onStartEdit={() => startEdit(def)}
          onSave={() => saveField(def)}
          onCancel={cancelEdit}
          onKeydown={(e) => handleKeydown(e, def)}
          onToggle={() => toggleBoolean(def)}
          onEditValueChange={(v) => {
            editValue = v
          }}
        />
      {/each}
    </div>
  {/if}

  {#if orphanedValues.length > 0}
    <div class="mt-4">
      <button
        class="flex items-center gap-1 text-sm text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
        onclick={() => {
          showDeprecated = !showDeprecated
        }}
      >
        {#if showDeprecated}
          <ChevronDown class="h-3.5 w-3.5" />
        {:else}
          <ChevronRight class="h-3.5 w-3.5" />
        {/if}
        {m.fields_deprecated()} ({orphanedValues.length})
      </button>

      {#if showDeprecated}
        <div class="mt-2 space-y-1.5">
          {#each orphanedValues as fv}
            <div
              class="rounded-lg border border-dashed border-gray-200 px-3 py-2 dark:border-gray-700"
            >
              <p
                class="text-xs font-semibold tracking-widest text-gray-300 uppercase dark:text-gray-600"
              >
                {fv.name}
                <span class="ml-1 text-gray-300 normal-case dark:text-gray-600"
                  >({m.deleted()})</span
                >
              </p>
              <p class="text-md mt-0.5 text-gray-400 dark:text-gray-500">
                {displayValue(fv)}
              </p>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>
