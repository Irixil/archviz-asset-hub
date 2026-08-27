<script lang="ts">
  interface Props {
    value?: string
    placeholder?: string
    onsubmit: (value: string) => void
    oncancel: () => void
    busy?: boolean
    size?: 'sm' | 'md'
    autofocus?: boolean
  }

  let {
    value = $bindable(''),
    placeholder = '',
    onsubmit,
    oncancel,
    busy = false,
    size = 'md',
    autofocus = true,
  }: Props = $props()

  function handleSubmit(e: SubmitEvent) {
    e.preventDefault()
    const trimmed = value.trim()
    if (trimmed) onsubmit(trimmed)
  }

  function handleBlur() {
    const trimmed = value.trim()
    if (trimmed) onsubmit(trimmed)
    else oncancel()
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.stopPropagation()
      oncancel()
    }
  }
</script>

<form class="flex items-center gap-1" onsubmit={handleSubmit}>
  <!-- svelte-ignore a11y_autofocus -->
  <input
    bind:value
    {placeholder}
    {autofocus}
    disabled={busy}
    onblur={handleBlur}
    onkeydown={handleKeydown}
    class="min-w-0 flex-1 rounded border border-indigo-400 px-1.5 py-0.5
      focus:ring-1 focus:ring-indigo-300 focus:outline-none
      disabled:opacity-50 dark:border-indigo-500 dark:bg-gray-800 dark:text-gray-100
      dark:focus:ring-indigo-600
      {size === 'sm' ? 'text-sm' : 'text-md'}"
  />
</form>
