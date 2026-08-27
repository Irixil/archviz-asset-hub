<script lang="ts">
  import { m } from '$lib/paraglide/messages'
  import { Download } from '@lucide/svelte'

  let {
    resourceUrl,
    text = m.download(),
  }: { resourceUrl: string; text: string } = $props()
</script>

<a
  href={resourceUrl}
  class="text-md flex items-center gap-2 rounded-lg border border-zinc-200 px-3 py-1.5 text-zinc-600 transition-colors
           hover:bg-zinc-50 dark:border-zinc-800 dark:text-zinc-400 dark:hover:bg-zinc-800"
>
  <Download class="h-4 w-4" />
  {text}
</a>
