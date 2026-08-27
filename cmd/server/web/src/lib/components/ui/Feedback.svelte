<script lang="ts">
  let {
    class: extraClass = '',
    success = undefined,
    error = undefined,
  }: {
    class?: string | null | undefined
    success?: string | null | undefined
    error?: string | null | undefined
  } = $props()
</script>

{#if error}
  <div
    class="text-md mx-5 mt-4 rounded-lg bg-red-50 px-4 py-3 text-red-700 dark:bg-red-900/30 dark:text-red-300 {extraClass}"
  >
    {error}
  </div>
{/if}
{#if success}
  <div
    class="text-md mx-5 mt-4 rounded-lg bg-green-50 px-4 py-3 text-green-700 dark:bg-green-900/30 dark:text-green-300 {extraClass}"
  >
    {success}
  </div>
{/if}
