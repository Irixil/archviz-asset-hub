<script lang="ts">
  import type { Snippet } from 'svelte'
  import Feedback from './Feedback.svelte'

  interface Props {
    value?: string
    label?: string
    error?: string
    type?: string
    placeholder?: string
    disabled?: boolean
    required?: boolean
    autocomplete?: AutoFill
    id?: string
    autofocus?: boolean
    leading?: Snippet
    trailing?: Snippet
    class?: string
    onchange?: (e: Event) => void
    oninput?: (e: Event) => void
    onblur?: (e: FocusEvent) => void
  }

  let {
    value = $bindable(''),
    label,
    error,
    type = 'text',
    placeholder,
    disabled = false,
    required = false,
    autocomplete,
    id = `input-${Math.random().toString(36).slice(2)}`,
    autofocus = false,
    leading,
    trailing,
    class: extraClass = '',
    onchange,
    oninput,
    onblur,
  }: Props = $props()
</script>

<div class="w-full {extraClass}">
  {#if label}
    <label
      for={id}
      class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
    >
      {label}
    </label>
  {/if}
  <div class="relative">
    {#if leading}
      <div
        class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-gray-400 dark:text-gray-500"
      >
        {@render leading()}
      </div>
    {/if}
    <!-- svelte-ignore a11y_autofocus -->
    <input
      {id}
      {type}
      {placeholder}
      {disabled}
      {required}
      {autocomplete}
      {autofocus}
      bind:value
      {onchange}
      {oninput}
      {onblur}
      class="text-md w-full rounded-lg border px-3 py-2 shadow-sm transition-colors
        focus:ring-2 focus:outline-none
        {error
        ? 'border-red-400 focus:ring-red-200 dark:border-red-500 dark:focus:ring-red-900'
        : 'border-gray-300 focus:border-indigo-400 focus:ring-indigo-200 dark:border-gray-600 dark:focus:border-indigo-500 dark:focus:ring-indigo-900'}
        bg-white text-gray-900 placeholder-gray-400
        disabled:cursor-not-allowed disabled:opacity-50 dark:bg-gray-800
        dark:text-gray-100 dark:placeholder-gray-500
        {leading ? 'pl-9' : ''}
        {trailing ? 'pr-9' : ''}"
    />
    {#if trailing}
      <div
        class="absolute top-1/2 right-3 -translate-y-1/2 text-gray-400 dark:text-gray-500"
      >
        {@render trailing()}
      </div>
    {/if}
  </div>
  <Feedback {error} />
</div>
