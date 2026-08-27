<script lang="ts">
  import type { FieldDefinition } from '$lib/api'
  import { Check, ArrowDownToLine } from '@lucide/svelte'
  import Feedback from './ui/Feedback.svelte'
  import { m } from '$lib/paraglide/messages'

  type FieldValue = {
    field_id: string
    field_type: string
    value: unknown
    definition_deleted: boolean
  }

  interface Props {
    def: FieldDefinition
    fv: FieldValue | undefined
    displayName?: string
    showUnset?: boolean
    isEditing: boolean
    isSaving: boolean
    didSave: boolean
    editValue: string
    saveError: string | null
    onStartEdit: () => void
    onSave: () => void
    onCancel: () => void
    onKeydown: (e: KeyboardEvent) => void
    onToggle: () => void
    onEditValueChange: (v: string) => void
  }

  let {
    def,
    fv,
    displayName,
    showUnset = false,
    isEditing,
    isSaving,
    didSave,
    editValue,
    saveError,
    onStartEdit,
    onSave,
    onCancel,
    onKeydown,
    onToggle,
    onEditValueChange,
  }: Props = $props()

  const label = $derived(displayName ?? def.name)

  function displayValue(fv: FieldValue): string {
    if (fv.value === null || fv.value === undefined) return ''
    if (fv.field_type === 'boolean') return fv.value ? m.yes() : m.no()
    return String(fv.value)
  }
</script>

<div
  class="rounded-xl border border-gray-100 bg-gray-50 px-3 py-2.5 dark:border-gray-800 dark:bg-gray-800/50"
>
  <div class="flex items-center justify-between gap-2">
    <div class="flex items-center gap-1.5">
      <p
        class="text-xs font-semibold tracking-widest text-gray-400 uppercase dark:text-gray-500"
      >
        {label}
        {#if def.required && (!fv || fv.value === null)}
          <span class="ml-1 text-orange-400">*</span>
        {/if}
      </p>
      {#if def.inherit_from_project}
        <span
          class="inline-flex items-center gap-0.5 rounded bg-indigo-50 px-1 py-0.5 text-[9px] font-medium text-indigo-700 dark:bg-indigo-950/40 dark:text-indigo-400"
          title={m.auto_fill_assets_desc()}
        >
          <ArrowDownToLine class="h-2.5 w-2.5" />
          {m.auto_fill_assets()}
        </span>
      {/if}
    </div>
    {#if didSave}
      <Check class="h-3.5 w-3.5 shrink-0 text-emerald-500" />
    {/if}
  </div>

  {#if isEditing}
    <div class="mt-1.5">
      {#if def.field_type === 'boolean'}
        <div class="flex gap-3">
          <label
            class="text-md flex items-center gap-1.5 text-gray-700 dark:text-gray-300"
          >
            <input
              type="radio"
              checked={editValue === 'true'}
              onchange={() => onEditValueChange('true')}
              class="text-indigo-600"
            />
            {m.yes()}
          </label>
          <label
            class="text-md flex items-center gap-1.5 text-gray-700 dark:text-gray-300"
          >
            <input
              type="radio"
              checked={editValue === 'false'}
              onchange={() => onEditValueChange('false')}
              class="text-indigo-600"
            />
            {m.no()}
          </label>
          {#if showUnset}
            <label
              class="text-md flex items-center gap-1.5 text-gray-700 dark:text-gray-300"
            >
              <input
                type="radio"
                checked={editValue === ''}
                onchange={() => onEditValueChange('')}
                disabled
                class="text-indigo-600"
              />
              {m.unset()}
            </label>
          {/if}
        </div>
      {:else if def.field_type === 'select'}
        <select
          value={editValue}
          onchange={(e) =>
            onEditValueChange((e.target as HTMLSelectElement).value)}
          class="text-md w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-gray-900
            focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none
            dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
        >
          <option value="">— clear —</option>
          {#each def.options ? (JSON.parse(def.options) as string[]) : [] as opt}
            <option value={opt}>{opt}</option>
          {/each}
        </select>
      {:else if def.field_type === 'date'}
        <input
          type="date"
          value={editValue}
          oninput={(e) =>
            onEditValueChange((e.target as HTMLInputElement).value)}
          onkeydown={onKeydown}
          class="text-md w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-gray-900
            focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none
            dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
        />
      {:else if def.field_type === 'number'}
        <input
          type="number"
          step="any"
          value={editValue}
          oninput={(e) =>
            onEditValueChange((e.target as HTMLInputElement).value)}
          onkeydown={onKeydown}
          class="text-md w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-gray-900
            focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none
            dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
        />
      {:else if def.field_type === 'url'}
        <input
          type="url"
          value={editValue}
          placeholder="https://"
          oninput={(e) =>
            onEditValueChange((e.target as HTMLInputElement).value)}
          onkeydown={onKeydown}
          class="text-md w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-gray-900
            focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none
            dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
        />
      {:else}
        <input
          type="text"
          value={editValue}
          oninput={(e) =>
            onEditValueChange((e.target as HTMLInputElement).value)}
          onkeydown={onKeydown}
          class="text-md w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-gray-900
            focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none
            dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
        />
      {/if}

      <Feedback error={saveError} />

      <div class="mt-1.5 flex gap-2">
        <button
          class="text-sm font-medium text-indigo-600 hover:text-indigo-800 disabled:opacity-50 dark:text-indigo-400 dark:hover:text-indigo-300"
          disabled={isSaving}
          onclick={onSave}
        >
          {isSaving ? m.saving() : m.save()}
        </button>
        <button
          class="text-sm text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          onclick={onCancel}
        >
          {m.cancel()}
        </button>
      </div>
    </div>
  {:else if def.field_type === 'boolean' && fv}
    <button
      class="text-md mt-1 flex items-center gap-2 font-medium
        {fv.value
        ? 'text-emerald-600 dark:text-emerald-400'
        : 'text-gray-400 dark:text-gray-500'}"
      onclick={onToggle}
    >
      <span
        class="h-4 w-7 rounded-full transition-colors {fv.value
          ? 'bg-emerald-500'
          : 'bg-gray-300 dark:bg-gray-600'} relative"
      >
        <span
          class="absolute top-0.5 h-3 w-3 rounded-full bg-white shadow transition-transform {fv.value
            ? 'left-3.5'
            : 'left-0.5'}"
        ></span>
      </span>
      {fv.value ? m.yes : m.no()}
    </button>
  {:else if fv && fv.value !== null && fv.value !== undefined}
    <button
      class="mt-1 flex w-full items-center justify-between text-left"
      onclick={onStartEdit}
    >
      <span
        class="text-md font-semibold text-gray-900 dark:text-gray-100 {def.field_type ===
        'url'
          ? 'truncate text-indigo-600 dark:text-indigo-400'
          : ''}"
      >
        {displayValue(fv)}
      </span>
      <span
        class="shrink-0 text-sm text-gray-400 opacity-0 transition-opacity hover:opacity-100"
        >{m.edit()}</span
      >
    </button>
  {:else}
    <button
      class="mt-1 text-sm text-gray-400 hover:text-indigo-600 dark:hover:text-indigo-400"
      onclick={onStartEdit}
    >
      {m.add_value()}
    </button>
  {/if}
</div>
