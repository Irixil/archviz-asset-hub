<script lang="ts">
  import { Clock, Zap } from '@lucide/svelte'
  import type { Workflow } from '$lib/api/workflows'
  import StatusBadge from '$lib/components/ui/StatusBadge.svelte'
  import SectionHeading from '$lib/components/ui/SectionHeading.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'
  import ButtonStart from '$lib/components/ui/ButtonStart.svelte'
  import ButtonToggle from '$lib/components/ui/ButtonToggle.svelte'
  import ButtonEdit from '$lib/components/ui/ButtonEdit.svelte'
  import ButtonDelete from '$lib/components/ui/ButtonDelete.svelte'

  interface Props {
    workflow: Workflow
    selected?: boolean
    onrun: (workflow: Workflow) => void
    ontoggle: (workflow: Workflow) => void
    onedit: (workflow: Workflow) => void
    ondelete: (workflow: Workflow) => void
    onclick: (workflow: Workflow) => void
  }

  let {
    workflow,
    selected = false,
    onrun,
    ontoggle,
    onedit,
    ondelete,
    onclick,
  }: Props = $props()

  function formatRelative(iso: string | null): string {
    if (!iso) return 'Never'
    const diff = Date.now() - new Date(iso).getTime()
    const mins = Math.floor(diff / 60_000)
    if (mins < 1) return 'just now'
    if (mins < 60) return `${mins}m ago`
    const hrs = Math.floor(mins / 60)
    if (hrs < 24) return `${hrs}h ago`
    return `${Math.floor(hrs / 24)}d ago`
  }

  function triggerLabel(type: string): string {
    return type.replace('trigger_', '').replace(/_/g, ' ')
  }
</script>

<div
  role="button"
  tabindex="0"
  class="overflow-hidden rounded-xl border bg-white transition-colors dark:bg-gray-900
    {selected
    ? 'border-indigo-300 dark:border-indigo-700'
    : 'border-zinc-200 hover:border-zinc-300 dark:border-zinc-800 dark:hover:border-zinc-700'}"
  onclick={() => onclick(workflow)}
  onkeydown={(e) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      onclick(workflow)
    }
  }}
>
  <div class="flex items-start gap-4 px-5 py-4">
    <!-- Icon -->
    <div
      class="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-violet-50 text-violet-500 dark:bg-violet-900/30 dark:text-violet-400"
    >
      <Zap class="h-5 w-5" />
    </div>

    <!-- Meta -->
    <div class="min-w-0 flex-1">
      <div class="flex flex-wrap items-center gap-2">
        <SectionHeading title={workflow.name} />
        {#if workflow.trigger_type}
          <Hint>{triggerLabel(workflow.trigger_type)}</Hint>
        {/if}
        <StatusBadge status={workflow.enabled ? 'healthy' : 'disabled'} />
      </div>

      <div
        class="mt-1 flex flex-wrap items-center gap-3 text-sm text-gray-400 dark:text-gray-500"
      >
        <span class="inline-flex items-center gap-1">
          <Clock class="h-3 w-3" />
          {formatRelative(workflow.last_run_at ?? null)}
        </span>
        {#if workflow.description}
          <span class="truncate">{workflow.description}</span>
        {/if}
      </div>
    </div>

    <!-- Actions -->
    <div
      class="flex shrink-0 items-center gap-1"
      role="none"
      onclick={(e) => e.stopPropagation()}
    >
      <ButtonStart
        onclick={() => onrun(workflow)}
        disabled={!workflow.enabled}
        title="Run now"
      />
      <ButtonToggle
        enabled={workflow.enabled}
        onclick={() => ontoggle(workflow)}
        title={workflow.enabled ? 'Pause' : 'Enable'}
      />
      <ButtonEdit onclick={() => onedit(workflow)} title="Edit" />
      <ButtonDelete onclick={() => ondelete(workflow)} title="Delete" />
    </div>
  </div>
</div>
