<script lang="ts">
  import { m } from '$lib/paraglide/messages'
  import { Search } from '@lucide/svelte'

  interface Props {
    value?: string
    placeholder?: string
    onchange?: (value: string) => void
    class?: string
  }

  let {
    value = $bindable(''),
    placeholder = m.search_(),
    onchange,
    class: extraClass = '',
  }: Props = $props()
</script>

<div class="relative {extraClass}">
  <Search
    class="pointer-events-none absolute top-1/2 left-3.5 h-4 w-4 -translate-y-1/2 text-[var(--text-muted)]"
  />
  <input
    type="search"
    data-search
    {placeholder}
    bind:value
    oninput={(e) => {
      value = e.currentTarget.value
      onchange?.(value)
    }}
    class="h-10 w-full rounded-md border border-[var(--border-default)] bg-[var(--bg-app)] pr-3 pl-10 text-sm text-[var(--text-primary)] shadow-inner placeholder:text-[var(--text-muted)] focus:border-[var(--accent)] focus:bg-[var(--bg-surface)] focus:ring-2 focus:ring-[var(--accent-soft)] focus:outline-none"
  />
</div>
