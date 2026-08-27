<script lang="ts">
  import { toastStore } from '$lib/stores/toast.svelte'
</script>

{#if toastStore.current}
  <div
    class="text-md fixed right-4 bottom-4 z-500 flex items-center gap-3 rounded-lg px-4 py-3 font-medium shadow-lg transition-colors
      {toastStore.current.type === 'error'
      ? 'bg-red-600 text-white'
      : 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900'}"
  >
    <span>{toastStore.current.msg}</span>
    {#if toastStore.current.action}
      <button
        onclick={() => {
          toastStore.current?.action?.onClick()
          toastStore.dismiss()
        }}
        class="shrink-0 rounded px-2 py-0.5 text-sm font-semibold underline-offset-2 hover:underline
          {toastStore.current.type === 'error'
          ? 'text-white'
          : 'text-indigo-300 dark:text-indigo-600'}"
      >
        {toastStore.current.action.label}
      </button>
    {/if}
  </div>
{/if}
