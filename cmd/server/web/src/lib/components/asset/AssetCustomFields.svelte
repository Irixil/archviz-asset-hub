<script lang="ts">
  import { assetFieldApi, fieldDefinitionApi } from '$lib/api/custom_fields'
  import type { Asset, AssetFieldValue, FieldDefinition } from '$lib/api'
  import { ChevronDown, ChevronRight, Camera } from '@lucide/svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import SubSectionTitle from '$lib/components/ui/SubSectionTitle.svelte'
  import FieldCard from '$lib/components/FieldCard.svelte'
  import { m } from '$lib/paraglide/messages'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { SetAssetField } from '$lib/commands/SetField'
  import { customFieldsStore } from '$lib/stores/customFields.svelte'

  interface Props {
    asset: Asset
  }

  let { asset }: Props = $props()

  let definitions = $state<readonly FieldDefinition[]>([])
  let values = $state<readonly AssetFieldValue[]>([])
  let loading = $state(true)
  let showDeprecated = $state(false)
  let showAllExif = $state(false)
  let showAllMediaTags = $state(false)

  let editingFieldId = $state<string | null>(null)
  let editValue = $state<string>('')
  let savingFieldId = $state<string | null>(null)
  let saveSuccess = $state<string | null>(null)
  let saveError = $state<string | null>(null)

  $effect(() => {
    if (asset.id) load()
  })

  async function load() {
    loading = true
    try {
      const [defs, vals] = await Promise.all([
        fieldDefinitionApi.list('asset'),
        assetFieldApi.get(asset.id),
      ])
      const userValues = vals.fields.filter(
        (field) => !field.source || field.source === 'user'
      )
      definitions = defs
      values = vals.fields
      customFieldsStore.setFieldValues(asset.id, userValues)
    } finally {
      loading = false
    }
  }

  function valueFor(fieldId: string): AssetFieldValue | undefined {
    return values.find((v) => v.field_id === fieldId)
  }

  function userValuesOnly(items: readonly AssetFieldValue[]) {
    return items.filter((field) => !field.source || field.source === 'user')
  }

  function hasValue(field: AssetFieldValue): boolean {
    return (
      field.value !== null && field.value !== undefined && field.value !== ''
    )
  }

  function displayLabel(field: AssetFieldValue): string {
    return field.name
      .replace(/^_exif_/, '')
      .replace(/^_media_/, '')
      .replace(/_/g, ' ')
  }

  function displayValue(fv: AssetFieldValue): string {
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
      editValue = ''
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

      const before = (valueFor(def.id)?.value ?? null) as
        | string
        | number
        | boolean
        | null
      if (parsedValue === null) {
        const result = await assetFieldApi.patch(asset.id, [
          { field_id: def.id, value: null },
        ])
        values = result.fields
        customFieldsStore.setFieldValues(
          asset.id,
          userValuesOnly(result.fields)
        )
      } else if (parsedValue !== before) {
        await undoStore.execute(
          new SetAssetField(asset.id, def.id, def.name, before, parsedValue)
        )
        const refreshed = await assetFieldApi.get(asset.id)
        values = refreshed.fields
        customFieldsStore.setFieldValues(
          asset.id,
          userValuesOnly(refreshed.fields)
        )
      }

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
  const exifValues = $derived(
    values.filter(
      (field) =>
        (field.source === 'exif' || field.key.startsWith('_exif_')) &&
        !field.definition_deleted
    )
  )
  const mediaTagValues = $derived(
    values.filter(
      (field) =>
        (field.source === 'media_tags' || field.key.startsWith('_media_')) &&
        !field.definition_deleted
    )
  )
  const orphanedValues = $derived(values.filter((v) => v.definition_deleted))
</script>

<div>
  <SubSectionTitle>{m.custom_fields_title()}</SubSectionTitle>

  {#if loading}
    <div class="flex justify-center py-4">
      <Spinner size="sm" />
    </div>
  {:else if activeDefinitions.length === 0 && exifValues.length === 0 && mediaTagValues.length === 0}
    <p class="text-sm text-gray-400 dark:text-gray-500">
      {m.no_custom_fields_yet()}
    </p>
  {:else}
    <div class="space-y-2">
      {#each activeDefinitions as def (def.id)}
        <FieldCard
          {def}
          fv={valueFor(def.id)}
          showUnset
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

    {#if exifValues.length > 0}
      <div class="mt-5">
        <div class="mb-2 flex items-center justify-between gap-1.5">
          <Camera class="h-3.5 w-3.5 text-gray-400 dark:text-gray-500" />
          <span
            class="text-xs font-semibold tracking-widest text-gray-400 uppercase dark:text-gray-500"
            >EXIF</span
          >
          <button
            class="ml-auto text-sm text-indigo-600 hover:underline dark:text-indigo-400"
            onclick={() => {
              showAllExif = !showAllExif
            }}
          >
            {m.all()}
          </button>
        </div>
        <div class="space-y-2">
          {#each exifValues as field (field.field_id)}
            {#if hasValue(field) || showAllExif}
              <div
                class="rounded-xl border border-gray-100 bg-gray-50 px-3 py-2.5 dark:border-gray-800 dark:bg-gray-800/50"
              >
                <p
                  class="text-xs font-semibold tracking-widest text-gray-400 uppercase dark:text-gray-500"
                >
                  {displayLabel(field)}
                </p>
                <p
                  class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100"
                >
                  {hasValue(field) ? displayValue(field) : m.unset()}
                </p>
              </div>
            {/if}
          {/each}
        </div>
      </div>
    {/if}

    {#if mediaTagValues.length > 0}
      <div class="mt-5">
        <div class="mb-2 flex items-center justify-between gap-1.5">
          <span
            class="text-xs font-semibold tracking-widest text-gray-400 uppercase dark:text-gray-500"
            >{m.media_tags_tab_label()}</span
          >
          <button
            class="ml-auto text-sm text-indigo-600 hover:underline dark:text-indigo-400"
            onclick={() => {
              showAllMediaTags = !showAllMediaTags
            }}
          >
            {m.all()}
          </button>
        </div>
        <div class="space-y-2">
          {#each mediaTagValues as field (field.field_id)}
            {#if hasValue(field) || showAllMediaTags}
              <div
                class="rounded-xl border border-gray-100 bg-gray-50 px-3 py-2.5 dark:border-gray-800 dark:bg-gray-800/50"
              >
                <p
                  class="text-xs font-semibold tracking-widest text-gray-400 uppercase dark:text-gray-500"
                >
                  {displayLabel(field)}
                </p>
                <p
                  class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100"
                >
                  {hasValue(field) ? displayValue(field) : m.unset()}
                </p>
              </div>
            {/if}
          {/each}
        </div>
      </div>
    {/if}
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
