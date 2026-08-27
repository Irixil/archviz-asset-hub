<script lang="ts">
  import { onMount } from 'svelte'
  import { AlertCircle, RefreshCw } from '@lucide/svelte'
  import { goto } from '$app/navigation'
  import { workflowsApi, type WorkflowTemplate } from '$lib/api/workflows'
  import { m } from '$lib/paraglide/messages'
  import Button from '$lib/components/ui/Button.svelte'
  import GridSkeleton from '$lib/components/ui/GridSkeleton.svelte'
  import TemplateCard from './TemplateCard.svelte'

  let templates = $state<WorkflowTemplate[]>([])
  let loadState = $state<'loading' | 'ready' | 'error'>('loading')
  let creatingId = $state<string | null>(null)
  let createErrorId = $state<string | null>(null)

  const orderedTemplates = $derived(
    [...templates].sort((a, b) =>
      a.id === 'blank-manual' ? -1 : b.id === 'blank-manual' ? 1 : 0
    )
  )

  onMount(() => void load())

  async function load() {
    loadState = 'loading'
    createErrorId = null
    try {
      templates = await workflowsApi.getTemplates()
      loadState = 'ready'
    } catch {
      loadState = 'error'
    }
  }

  async function selectTemplate(t: WorkflowTemplate) {
    creatingId = t.id
    createErrorId = null
    try {
      const created = await workflowsApi.create({
        name: t.name,
        description: t.description,
        graph: t.graph,
      })
      await goto(`/library/settings/workflows?workflow=${created.id}`)
    } catch {
      createErrorId = t.id
    } finally {
      creatingId = null
    }
  }
</script>

{#if loadState === 'loading'}
  <GridSkeleton lines={3} />
{:else if loadState === 'error'}
  <div
    class="flex min-h-40 flex-col items-center justify-center gap-3 text-sm text-gray-600 dark:text-gray-300"
  >
    <AlertCircle class="h-5 w-5 text-red-500" />
    <p>{m.workflows_new_page_load_error()}</p>
    <Button variant="secondary" size="sm" onclick={() => void load()}>
      {#snippet icon()}<RefreshCw class="h-4 w-4" />{/snippet}
      {m.retry()}
    </Button>
  </div>
{:else}
  <div
    class="grid gap-4"
    style="grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));"
  >
    {#each orderedTemplates as template (template.id)}
      <div class="flex h-full flex-col gap-2">
        <TemplateCard
          icon={template.icon}
          title={template.name}
          description={template.description}
          badge={template.category || null}
          loading={creatingId === template.id}
          disabled={creatingId !== null && creatingId !== template.id}
          onSelect={() => void selectTemplate(template)}
        />
        {#if createErrorId === template.id}
          <p class="text-xs text-red-500">
            {m.workflows_new_page_create_error()}
          </p>
        {/if}
      </div>
    {/each}
  </div>
{/if}
