<script lang="ts">
  import { assetFieldApi, fieldDefinitionApi } from '$lib/api'
  import type { BulkFieldPreviewEntry, FieldDefinition } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import { undoStore } from '$lib/stores/undo.svelte'
  import {
    BulkMetadataCommand,
    type FieldEdit,
    type TagEdit,
  } from '$lib/commands/BulkMetadataCommand'
  import BulkTagRow from './BulkTagRow.svelte'
  import BulkFieldRow from './BulkFieldRow.svelte'
  import Button from './ui/Button.svelte'
  import { X } from '@lucide/svelte'

  type FieldAction = 'noop' | 'set' | 'clear'

  interface TagRow {
    id: number
    mode: 'add' | 'remove'
    tag: string
  }

  interface FieldRow {
    id: number
    fieldId: string | null
    value: string | number | boolean | null
    action: FieldAction
  }

  interface Props {
    assetIds: string[]
    onClose: () => void
    onCommit: () => void
  }

  let { assetIds, onClose, onCommit }: Props = $props()

  let nextId = 0
  let tagRows = $state<TagRow[]>([{ id: nextId++, mode: 'add', tag: '' }])
  let fieldRows = $state<FieldRow[]>([
    { id: nextId++, fieldId: null, value: null, action: 'noop' },
  ])

  let fieldDefs = $state<FieldDefinition[]>([])
  let preview = $state<Map<string, BulkFieldPreviewEntry>>(new Map())
  let loadingPreview = $state(false)
  let saving = $state(false)
  let validationError = $state<string | null>(null)
  let fieldDefsLoaded = $state(false)

  let previewDebounce: ReturnType<typeof setTimeout>

  async function loadFieldDefs() {
    if (fieldDefsLoaded) return
    try {
      const defs = await fieldDefinitionApi.list('asset')
      fieldDefs = defs.filter((d) => !d.deleted_at)
      fieldDefsLoaded = true
    } catch {
      /* ignore */
    }
  }

  function schedulePreview() {
    clearTimeout(previewDebounce)
    const activeFieldIds = fieldRows
      .filter((r) => r.fieldId !== null)
      .map((r) => r.fieldId as string)
    if (activeFieldIds.length === 0) {
      preview = new Map()
      return
    }
    previewDebounce = setTimeout(async () => {
      loadingPreview = true
      try {
        const res = await assetFieldApi.bulkPreview(assetIds, activeFieldIds)
        const map = new Map<string, BulkFieldPreviewEntry>()
        for (const entry of res.fields) map.set(entry.field_id, entry)
        preview = map
      } catch {
        /* ignore */
      } finally {
        loadingPreview = false
      }
    }, 300)
  }

  function updateFieldRow(
    rowId: number,
    fieldId: string | null,
    value: string | number | boolean | null,
    action: FieldAction
  ) {
    fieldRows = fieldRows.map((r) =>
      r.id === rowId ? { ...r, fieldId, value, action } : r
    )
    schedulePreview()
  }

  function addTagRow() {
    tagRows = [...tagRows, { id: nextId++, mode: 'add', tag: '' }]
  }

  function removeTagRow(id: number) {
    tagRows = tagRows.filter((r) => r.id !== id)
  }

  function addFieldRow() {
    fieldRows = [
      ...fieldRows,
      { id: nextId++, fieldId: null, value: null, action: 'noop' },
    ]
  }

  function removeFieldRow(id: number) {
    fieldRows = fieldRows.filter((r) => r.id !== id)
  }

  async function handleSubmit() {
    validationError = null

    const activeTagEdits: TagEdit[] = tagRows
      .filter((r) => r.tag.trim() !== '')
      .map((r) => ({ tag: r.tag.trim().toLowerCase(), mode: r.mode }))

    const activeFieldEdits: FieldEdit[] = fieldRows
      .filter((r) => r.fieldId !== null && r.action !== 'noop')
      .map((r) => ({
        fieldId: r.fieldId as string,
        value: r.action === 'clear' ? null : r.value,
      }))

    if (activeTagEdits.length === 0 && activeFieldEdits.length === 0) {
      validationError = m.bulk_validation_empty()
      return
    }

    saving = true
    try {
      const previewSnapshot = {
        fields: activeFieldEdits.length > 0 ? [...preview.values()] : [],
      }
      await undoStore.execute(
        new BulkMetadataCommand(
          assetIds,
          activeTagEdits,
          activeFieldEdits,
          previewSnapshot
        )
      )
      onCommit()
    } catch {
      validationError = m.try_again()
    } finally {
      saving = false
    }
  }

  // Load field defs when modal mounts.
  $effect(() => {
    loadFieldDefs()
  })
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm"
  onclick={(e) => e.target === e.currentTarget && onClose()}
>
  <div
    class="flex w-full max-w-lg flex-col rounded-2xl border border-gray-200 bg-white shadow-2xl dark:border-gray-700 dark:bg-gray-900"
    style="max-height: 90vh; overflow-y: auto;"
  >
    <!-- Header -->
    <div
      class="sticky top-0 z-10 flex shrink-0 items-center justify-between border-b border-gray-100 bg-white px-5 py-4 dark:border-gray-800 dark:bg-gray-900"
    >
      <h2 class="text-base font-semibold text-gray-800 dark:text-gray-100">
        {m.bulk_metadata_modal_title({ count: String(assetIds.length) })}
      </h2>
      <button
        type="button"
        class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
        onclick={onClose}
        aria-label="Close"
      >
        <X class="h-4 w-4" />
      </button>
    </div>

    <!-- Body -->
    <div class="px-5 py-4">
      <!-- Tags section -->
      <p
        class="mb-2 text-xs font-semibold tracking-wide text-gray-500 uppercase dark:text-gray-400"
      >
        {m.bulk_metadata_tags_section()}
      </p>
      <div class="mb-3 flex flex-col gap-2">
        {#each tagRows as row (row.id)}
          <BulkTagRow
            bind:mode={row.mode}
            bind:tag={row.tag}
            onRemoveRow={() => removeTagRow(row.id)}
          />
        {/each}
        <button
          type="button"
          class="mt-1 text-left text-xs text-blue-500 hover:underline dark:text-blue-400"
          onclick={addTagRow}
        >
          + {m.bulk_metadata_add_tag()}
        </button>
      </div>

      <!-- Custom fields section -->
      <p
        class="mt-4 mb-2 text-xs font-semibold tracking-wide text-gray-500 uppercase dark:text-gray-400"
      >
        {m.bulk_metadata_fields_section()}
        {#if loadingPreview}
          <span class="ml-1 animate-pulse text-gray-300">…</span>
        {/if}
      </p>
      <div class="mb-3 flex flex-col gap-3">
        {#each fieldRows as row (row.id)}
          <BulkFieldRow
            {fieldDefs}
            fieldId={row.fieldId}
            value={row.value}
            action={row.action}
            preview={row.fieldId ? (preview.get(row.fieldId) ?? null) : null}
            onUpdate={(fid, val, act) => updateFieldRow(row.id, fid, val, act)}
            onRemoveRow={() => removeFieldRow(row.id)}
          />
        {/each}
        <button
          type="button"
          class="mt-1 text-left text-xs text-blue-500 hover:underline dark:text-blue-400"
          onclick={addFieldRow}
        >
          + {m.bulk_metadata_add_field()}
        </button>
      </div>

      {#if validationError}
        <p class="mt-2 text-xs text-rose-500">{validationError}</p>
      {/if}
    </div>

    <!-- Footer -->
    <div
      class="sticky bottom-0 flex items-center justify-end gap-2 border-t border-gray-100 bg-white px-5 py-3 dark:border-gray-800 dark:bg-gray-900"
    >
      <Button variant="ghost" size="sm" disabled={saving} onclick={onClose}>
        {m.cancel()}
      </Button>
      <Button
        variant="primary"
        size="sm"
        disabled={saving}
        onclick={handleSubmit}
      >
        {saving
          ? '…'
          : m.bulk_metadata_apply({ count: String(assetIds.length) })}
      </Button>
    </div>
  </div>
</div>
