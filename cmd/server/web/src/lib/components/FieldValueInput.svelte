<script lang="ts">
  import type { FieldType } from '$lib/api'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    fieldType: FieldType
    value: string | number | boolean | null
    options?: string | null // JSON array string for 'select' type
    allowClear?: boolean
    onchange: (v: string | number | boolean | null) => void
  }

  let {
    fieldType,
    value,
    options = null,
    allowClear = false,
    onchange,
  }: Props = $props()

  const parsedOptions = $derived.by(() => {
    if (!options) return []
    try {
      return JSON.parse(options) as string[]
    } catch {
      return []
    }
  })

  function handleInput(e: Event) {
    const el = e.currentTarget as HTMLInputElement
    if (fieldType === 'number') {
      onchange(el.value === '' ? null : Number(el.value))
    } else {
      onchange(el.value)
    }
  }

  function handleSelect(e: Event) {
    const el = e.currentTarget as HTMLSelectElement
    if (el.value === '__clear__') {
      onchange(null)
    } else if (fieldType === 'boolean') {
      onchange(el.value === 'true')
    } else {
      onchange(el.value)
    }
  }

  const inputClass =
    'w-full rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-sm text-gray-800 focus:border-blue-400 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200'
  const selectClass = inputClass
</script>

{#if fieldType === 'boolean'}
  <select
    class={selectClass}
    value={value === null ? '__clear__' : String(value)}
    onchange={handleSelect}
  >
    {#if allowClear}
      <option value="__clear__">{m.bulk_metadata_clear_value()}</option>
    {/if}
    <option value="true">Yes</option>
    <option value="false">No</option>
  </select>
{:else if fieldType === 'select'}
  <select
    class={selectClass}
    value={value === null ? '__clear__' : String(value)}
    onchange={handleSelect}
  >
    {#if allowClear}
      <option value="__clear__">{m.bulk_metadata_clear_value()}</option>
    {/if}
    {#each parsedOptions as opt}
      <option value={opt}>{opt}</option>
    {/each}
  </select>
{:else if fieldType === 'date'}
  <div class="relative flex items-center gap-1">
    <input
      type="date"
      class={inputClass}
      value={value === null ? '' : String(value)}
      oninput={handleInput}
    />
    {#if allowClear && value !== null}
      <button
        type="button"
        class="shrink-0 text-xs text-gray-400 hover:text-gray-600"
        onclick={() => onchange(null)}
        aria-label="Clear value"
      >
        ×
      </button>
    {/if}
  </div>
{:else if fieldType === 'number'}
  <div class="relative flex items-center gap-1">
    <input
      type="number"
      class={inputClass}
      value={value === null ? '' : String(value)}
      oninput={handleInput}
    />
    {#if allowClear && value !== null}
      <button
        type="button"
        class="shrink-0 text-xs text-gray-400 hover:text-gray-600"
        onclick={() => onchange(null)}
        aria-label="Clear value"
      >
        ×
      </button>
    {/if}
  </div>
{:else if fieldType === 'url'}
  <div class="relative flex items-center gap-1">
    <input
      type="url"
      class={inputClass}
      value={value === null ? '' : String(value)}
      oninput={handleInput}
    />
    {#if allowClear && value !== null}
      <button
        type="button"
        class="shrink-0 text-xs text-gray-400 hover:text-gray-600"
        onclick={() => onchange(null)}
        aria-label="Clear value"
      >
        ×
      </button>
    {/if}
  </div>
{:else}
  <!-- text (default) -->
  <div class="relative flex items-center gap-1">
    <input
      type="text"
      class={inputClass}
      value={value === null ? '' : String(value)}
      oninput={handleInput}
    />
    {#if allowClear && value !== null}
      <button
        type="button"
        class="shrink-0 text-sm text-gray-400 hover:text-gray-600"
        onclick={() => onchange(null)}
        aria-label="Clear value"
      >
        ×
      </button>
    {/if}
  </div>
{/if}
