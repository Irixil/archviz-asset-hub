<script lang="ts">
  import { CircleAlert, CircleCheck } from '@lucide/svelte'
  let {
    i = undefined,
    status,
    text,
  }: {
    i?: boolean
    status: 'healthy' | 'error' | 'disabled'
    text?: string
  } = $props()
</script>

<!-- Status pill -->
{#if status === 'disabled'}
  <span
    class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-500 dark:bg-gray-800 dark:text-gray-400"
  >
    {#if i ?? false}
      <CircleAlert class="h-3 w-3" />
    {/if}
    {text || 'Disabled'}
  </span>
{:else if status === 'error'}
  <span
    class="inline-flex items-center gap-1 rounded-full bg-red-100 px-2 py-0.5 text-[11px] font-medium text-red-600 dark:bg-red-900/30 dark:text-red-400"
  >
    {#if i ?? true}
      <CircleAlert class="h-3 w-3" />
    {/if}
    {text || 'Error'}
  </span>
{:else if status === 'healthy'}
  <span
    class="inline-flex items-center gap-1 rounded-full bg-green-100 px-2 py-0.5 text-[11px] font-medium text-green-700 dark:bg-green-900/30 dark:text-green-400"
  >
    {#if i ?? true}
      <CircleCheck class="h-3 w-3" />
    {/if}
    {text || 'Healthy'}
  </span>
{/if}
