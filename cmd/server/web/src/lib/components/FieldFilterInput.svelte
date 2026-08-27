<script lang="ts">
  import type { FieldDefinition } from '$lib/api'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    def: FieldDefinition
    local: Record<string, unknown>
    onchange: () => void
    ondebouncedchange: () => void
  }

  let { def, local, onchange, ondebouncedchange }: Props = $props()
</script>

<div class="flex min-w-0 flex-col gap-1">
  <label
    for="field-{def.key}"
    class="text-xs font-semibold tracking-widest text-gray-400 uppercase dark:text-gray-500"
  >
    {def.name}
  </label>

  {#if def.field_type === 'text' || def.field_type === 'url'}
    <input
      id="field-{def.key}"
      type="text"
      placeholder={m.field_contains_ph()}
      value={local[def.key] as string}
      oninput={(e) => {
        local[def.key] = (e.target as HTMLInputElement).value
        ondebouncedchange()
      }}
      class="w-36 rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-sm text-gray-900
        focus:border-indigo-400 focus:outline-none
        dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
    />
  {:else if def.field_type === 'number'}
    {@const nums = {
      ...{ min: '', max: '' },
      ...(local[def.key] as { min: string; max: string }),
    }}
    <div class="flex items-center gap-1">
      <input
        type="number"
        step="any"
        placeholder="min"
        value={nums.min}
        oninput={(e) => {
          ;(local[def.key] as { min: string; max: string }).min = (
            e.target as HTMLInputElement
          ).value
          ondebouncedchange()
        }}
        class="w-20 rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-sm text-gray-900
          focus:border-indigo-400 focus:outline-none
          dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
      />
      <span class="text-sm text-gray-400">–</span>
      <input
        type="number"
        step="any"
        placeholder="max"
        value={nums.max}
        oninput={(e) => {
          ;(local[def.key] as { min: string; max: string }).max = (
            e.target as HTMLInputElement
          ).value
          ondebouncedchange()
        }}
        class="w-20 rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-sm text-gray-900
          focus:border-indigo-400 focus:outline-none
          dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
      />
    </div>
  {:else if def.field_type === 'date'}
    {@const dates = local[def.key] as { from: string; to: string }}
    <div class="flex items-center gap-1">
      <input
        type="date"
        value={dates?.from || ''}
        onchange={(e) => {
          ;(local[def.key] as { from: string; to: string }).from = (
            e.target as HTMLInputElement
          ).value
          onchange()
        }}
        class="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-sm text-gray-900
          focus:border-indigo-400 focus:outline-none
          dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
      />
      <span class="text-sm text-gray-400">–</span>
      <input
        type="date"
        value={dates?.to || ''}
        onchange={(e) => {
          ;(local[def.key] as { from: string; to: string }).to = (
            e.target as HTMLInputElement
          ).value
          onchange()
        }}
        class="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-sm text-gray-900
          focus:border-indigo-400 focus:outline-none
          dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
      />
    </div>
  {:else if def.field_type === 'boolean'}
    {@const bv = local[def.key] as string}
    <div class="flex gap-2">
      {#each [['', m.any()], ['true', m.yes()], ['false', m.no()]] as [val, label]}
        <button
          type="button"
          class="rounded-full border px-2.5 py-1 text-sm transition-colors
            {bv === val
            ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:border-indigo-400 dark:bg-indigo-950/40 dark:text-indigo-300'
            : 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-gray-700 dark:text-gray-400'}"
          onclick={() => {
            local[def.key] = val
            onchange()
          }}
        >
          {label}
        </button>
      {/each}
    </div>
  {:else if def.field_type === 'select'}
    {@const opts = def.options ? (JSON.parse(def.options) as string[]) : []}
    {@const sv = local[def.key] as string}
    <div class="flex flex-wrap gap-1">
      {#each opts as opt}
        <button
          type="button"
          class="rounded-full border px-2.5 py-1 text-sm transition-colors
            {sv === opt
            ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:border-indigo-400 dark:bg-indigo-950/40 dark:text-indigo-300'
            : 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-gray-700 dark:text-gray-400'}"
          onclick={() => {
            local[def.key] = sv === opt ? '' : opt
            onchange()
          }}
        >
          {opt}
        </button>
      {/each}
    </div>
  {/if}
</div>
