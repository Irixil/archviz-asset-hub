<script lang="ts">
  import { Check, X } from '@lucide/svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    label: string
    disabled?: boolean
    onaccept: () => void
    ondismiss: () => void
  }

  let { label, disabled = false, onaccept, ondismiss }: Props = $props()
</script>

<span
  class="inline-flex items-center gap-1 rounded-full border border-dashed border-indigo-400
    bg-indigo-50 px-2.5 py-1 text-sm text-indigo-700 dark:border-indigo-500 dark:bg-indigo-950
    dark:text-indigo-300"
  title={m.auto_tag_pending_title()}
>
  <span class="select-none opacity-70">✦</span>
  {label}

  <button
    type="button"
    onclick={onaccept}
    {disabled}
    aria-label={m.auto_tag_accept({ tag: label })}
    class="ml-0.5 rounded-full p-0.5 hover:bg-indigo-200 disabled:opacity-50 dark:hover:bg-indigo-800"
  >
    <Check class="h-3 w-3" />
  </button>

  <button
    type="button"
    onclick={ondismiss}
    {disabled}
    aria-label={m.auto_tag_dismiss({ tag: label })}
    class="rounded-full p-0.5 hover:bg-indigo-200 disabled:opacity-50 dark:hover:bg-indigo-800"
  >
    <X class="h-3 w-3" />
  </button>
</span>
