<script>
  import { m } from '$lib/paraglide/messages'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { Redo2, Undo2 } from '@lucide/svelte'
</script>

<div class="flex items-center gap-1">
  <button
    onclick={() => undoStore.undo()}
    disabled={!undoStore.canUndo}
    title={undoStore.undoLabel
      ? `${m.undo()}: ${undoStore.undoLabel}`
      : m.undo()}
    class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:opacity-30 dark:hover:bg-gray-800 dark:hover:text-gray-300"
    ><Undo2 class="h-4 w-4" /></button
  >
  <button
    onclick={() => undoStore.redo()}
    disabled={!undoStore.canRedo}
    title={undoStore.redoLabel
      ? `${m.redo()}: ${undoStore.redoLabel}`
      : m.redo()}
    class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:opacity-30 dark:hover:bg-gray-800 dark:hover:text-gray-300"
    ><Redo2 class="h-4 w-4" /></button
  >
</div>
