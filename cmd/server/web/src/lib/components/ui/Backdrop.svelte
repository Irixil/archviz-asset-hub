<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    onClose,
    children,
    class: extraClass,
  }: { onClose?: () => void; children?: Snippet; class?: string } = $props()
</script>

<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-md {extraClass}"
  role="button"
  tabindex="-1"
  onclick={(e) => {
    if (e.target === e.currentTarget) onClose?.()
  }}
  onkeydown={(e) => e.key === 'Enter' && onClose?.()}
  aria-label="Close modal"
>
  {@render children?.()}
</div>
