<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    open?: boolean
    onclose?: () => void
    side?: 'center' | 'right'
    children: Snippet
  }

  let {
    open = $bindable(false),
    onclose,
    side = 'center',
    children,
  }: Props = $props()

  function handleBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) {
      onclose?.()
      open = false
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      onclose?.()
      open = false
    }
  }
</script>

<svelte:window onkeydown={open ? handleKeydown : undefined} />

{#if open}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div
    class="fixed inset-0 z-50 flex
      {side === 'right' ? 'justify-end' : 'items-center justify-center'}
      bg-black/40 backdrop-blur-sm"
    onclick={handleBackdropClick}
  >
    <div
      class="relative bg-white shadow-xl dark:bg-gray-900
        {side === 'right'
        ? 'h-full w-full max-w-xl overflow-y-auto'
        : 'max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-2xl'}"
    >
      {@render children()}
    </div>
  </div>
{/if}
