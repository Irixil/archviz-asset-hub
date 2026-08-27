<script lang="ts">
  import type { Project } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { ChevronDown, ChevronRight, Box } from '@lucide/svelte'
  import ProjectCustomFields from './ProjectCustomFields.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    project: Project
  }

  let { project }: Props = $props()

  let expanded = $state(false)
</script>

<div class="border-b border-[var(--border-subtle)] bg-[var(--bg-surface)]">
  <button
    type="button"
    class="flex w-full items-center gap-2 px-6 py-2 text-left text-sm transition-colors hover:bg-[var(--bg-hover)]"
    onclick={() => {
      expanded = !expanded
    }}
    aria-expanded={expanded}
  >
    <Box
      class="h-3.5 w-3.5 shrink-0 text-[var(--text-muted)]"
      style="color: {project.color ?? 'var(--text-muted)'}"
    />
    <span class="flex-1 font-medium text-[var(--text-primary)]"
      >{project.name}</span
    >
    {#if project.description}
      <span class="max-w-xs truncate text-[var(--text-muted)]"
        >{project.description}</span
      >
    {/if}
    {#if expanded}
      <ChevronDown class="h-3.5 w-3.5 shrink-0 text-[var(--text-muted)]" />
    {:else}
      <ChevronRight class="h-3.5 w-3.5 shrink-0 text-[var(--text-muted)]" />
    {/if}
  </button>

  {#if expanded}
    <div class="border-t border-[var(--border-subtle)] px-6 py-4">
      {#if authStore.role !== 'viewer'}
        <ProjectCustomFields projectId={project.id} />
      {:else}
        <p class="text-sm text-[var(--text-muted)]">
          {m.project_details()}
        </p>
      {/if}
    </div>
  {/if}
</div>
