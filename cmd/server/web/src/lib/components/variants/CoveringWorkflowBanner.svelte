<script lang="ts">
  import type { CoveringWorkflow } from '$lib/api'
  import { Zap } from '@lucide/svelte'
  import { m } from '$lib/paraglide/messages'

  let { workflow }: { workflow: CoveringWorkflow } = $props()

  const scopeLabels: Record<CoveringWorkflow['scope'], string> = {
    workspace: m.variant_automation_covered_scope_workspace(),
    project: m.variant_automation_covered_scope_project(),
    folder: m.variant_automation_covered_scope_folder(),
    asset: m.variant_automation_covered_scope_asset(),
  }
</script>

<div
  class="mb-4 flex items-center gap-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-900 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-100"
  role="status"
>
  <Zap class="h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-300" />
  <div class="min-w-0">
    <div>{m.variant_automation_covered_by()}</div>
    <a
      class="font-medium text-emerald-950 underline decoration-emerald-400 underline-offset-2 hover:text-emerald-700 dark:text-emerald-50 dark:hover:text-emerald-200"
      href={workflow.workflow_url}
    >
      {workflow.name}
    </a>
    <span class="ml-2 text-xs text-emerald-700 dark:text-emerald-300">
      {scopeLabels[workflow.scope]}
    </span>
  </div>
</div>
