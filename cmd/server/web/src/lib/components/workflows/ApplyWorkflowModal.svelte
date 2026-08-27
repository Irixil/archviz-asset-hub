<script lang="ts">
  import { X } from '@lucide/svelte'
  import { m } from '$lib/paraglide/messages'
  import Modal from '$lib/components/ui/Modal.svelte'
  import ApplyWorkflowPicker from './ApplyWorkflowPicker.svelte'

  interface AppliedWorkflow {
    workflowId: string
    workflowName: string
    runIds: string[]
    partialError: string | undefined
  }

  interface Props {
    assetIds: string[]
    onClose: () => void
    onApplied: (results: AppliedWorkflow[]) => void
  }

  let { assetIds, onClose, onApplied }: Props = $props()

  function handleApplied(results: AppliedWorkflow[]) {
    onApplied(results)
    onClose()
  }
</script>

<Modal open={true} onclose={onClose}>
  <div
    class="flex items-center justify-between border-b border-gray-200 px-5 py-4 dark:border-gray-700"
  >
    <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">
      {m.workflows_library_modal_title()}
    </h2>
    <button
      type="button"
      class="rounded-md p-1 text-gray-500 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-100"
      onclick={onClose}
      aria-label="Close"
    >
      <X class="h-4 w-4" />
    </button>
  </div>

  <div class="px-5 py-5">
    <ApplyWorkflowPicker {assetIds} onApplied={handleApplied} />
  </div>
</Modal>
