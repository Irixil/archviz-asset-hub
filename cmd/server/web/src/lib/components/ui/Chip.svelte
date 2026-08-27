<script lang="ts">
  import { X } from '@lucide/svelte'

  interface Props {
    label: string
    onremove?: () => void
    onclick?: () => void
    color?: string
  }

  let { label, onremove, onclick, color }: Props = $props()
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<span
  {onclick}
  class="inline-flex cursor-pointer items-center gap-1 rounded-full bg-gray-100 px-3 py-1
    text-sm font-medium text-gray-700 dark:bg-gray-700 dark:text-gray-200"
  style={color ? `background-color: ${color}20; color: ${color}` : ''}
>
  {label}
  {#if onremove}
    <button
      type="button"
      onclick={onremove}
      class="ml-0.5 rounded-full p-0.5 hover:bg-black/10 focus:outline-none dark:hover:bg-white/10"
      aria-label="Remove {label}"
    >
      <X class="h-3 w-3" />
    </button>
  {/if}
</span>
