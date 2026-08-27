<script lang="ts">
  import type {
    BulkFieldPreviewEntry,
    FieldDefinition,
    FieldType,
  } from '$lib/api'
  import FieldValueInput from './FieldValueInput.svelte'
  import { m } from '$lib/paraglide/messages'
  import { Trash2, Undo2 } from '@lucide/svelte'

  type FieldAction = 'noop' | 'set' | 'clear'

  interface Props {
    fieldDefs: FieldDefinition[]
    fieldId: string | null
    value: string | number | boolean | null
    action: FieldAction
    preview: BulkFieldPreviewEntry | null
    onUpdate: (
      fieldId: string | null,
      value: string | number | boolean | null,
      action: FieldAction
    ) => void
    onRemoveRow: () => void
  }

  let {
    fieldDefs,
    fieldId,
    value,
    action,
    preview,
    onUpdate,
    onRemoveRow,
  }: Props = $props()

  const selectedDef = $derived(fieldDefs.find((d) => d.id === fieldId) ?? null)

  function handleFieldSelect(e: Event) {
    const el = e.currentTarget as HTMLSelectElement
    const id = el.value || null
    onUpdate(id, null, 'noop')
  }

  function handleValueChange(v: string | number | boolean | null) {
    if (v === null) {
      onUpdate(fieldId, null, 'clear')
    } else {
      onUpdate(fieldId, v, 'set')
    }
  }
</script>

<div class="flex flex-col gap-1.5">
  <div class="flex items-center gap-2">
    <!-- Field selector -->
    <select
      class="flex-1 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-sm text-gray-800 focus:border-blue-400 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
      value={fieldId ?? ''}
      onchange={handleFieldSelect}
    >
      <option value="">{m.bulk_field_select_placeholder()}</option>
      {#each fieldDefs as def}
        <option value={def.id}>{def.name}</option>
      {/each}
    </select>

    {#if selectedDef && action !== 'clear'}
      <span class="text-xs text-gray-400 dark:text-gray-500">=</span>
      <div class="flex-1">
        <FieldValueInput
          fieldType={selectedDef.field_type as FieldType}
          value={action === 'noop' ? null : value}
          options={selectedDef.options}
          allowClear={true}
          onchange={handleValueChange}
        />
      </div>
    {/if}

    {#if selectedDef && action === 'clear'}
      <div class="flex flex-1 items-center gap-1.5">
        <span
          class="inline-flex items-center gap-1 rounded-md border border-rose-200 bg-rose-50 px-2 py-0.5 text-xs font-medium text-rose-600 dark:border-rose-800/60 dark:bg-rose-950/40 dark:text-rose-400"
        >
          <span class="h-1.5 w-1.5 rounded-full bg-rose-400 dark:bg-rose-500"
          ></span>
          {m.bulk_metadata_clear_value()}
        </span>
        <button
          type="button"
          class="flex h-5 w-5 shrink-0 items-center justify-center rounded text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-300"
          aria-label="Undo clear"
          onclick={() => onUpdate(fieldId, null, 'noop')}
        >
          <Undo2 class="h-3 w-3" />
        </button>
      </div>
    {/if}

    <button
      type="button"
      class="flex h-6 w-6 shrink-0 items-center justify-center rounded text-gray-300 transition-colors hover:bg-rose-50 hover:text-rose-500 dark:text-gray-600 dark:hover:bg-rose-950/40 dark:hover:text-rose-400"
      aria-label="Remove row"
      onclick={onRemoveRow}
    >
      <Trash2 class="h-3.5 w-3.5" />
    </button>
  </div>

  <!-- Preview hint -->
  {#if preview && action !== 'noop' && preview.assets_with_value > 0}
    <p class="ml-1 text-xs text-amber-600 dark:text-amber-400">
      {m.bulk_field_overwrite_hint({
        values: preview.distinct_values.join(', '),
        count: String(preview.assets_with_value),
      })}
    </p>
  {:else if selectedDef && action === 'noop'}
    <p class="ml-1 text-xs text-gray-400">
      {m.bulk_metadata_no_change_placeholder()}
    </p>
  {/if}
</div>
