<script lang="ts">
  import { type Asset, type Project } from '$lib/api'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import ColorDot from '$lib/components/ui/ColorDot.svelte'
  import { m } from '$lib/paraglide/messages'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { AssignAssetToProject } from '$lib/commands/AssignAssetToProject'

  interface Props {
    asset: Asset
    activeProject: Project | null
  }

  let { asset, activeProject }: Props = $props()

  let showProjectPicker = $state(false)

  async function assignProject(projectId: string | null) {
    if (!asset) return
    if (projectId === asset.project_id) {
      showProjectPicker = false
      return
    }
    const afterProject =
      projectsStore.projects.find((p) => p.id === projectId) ?? null
    try {
      await undoStore.execute(
        new AssignAssetToProject(
          asset.id,
          asset.project_id ?? null,
          activeProject?.name ?? null,
          projectId,
          afterProject?.name ?? null
        )
      )
      showProjectPicker = false
    } catch {
      /* silently ignore */
    }
  }
</script>

<div>
  <div class="mb-3 flex items-center justify-between">
    <p
      class="text-xs font-semibold tracking-widest text-gray-400 uppercase dark:text-gray-500"
    >
      {m.project()}
    </p>
    {#if authStore.role !== 'viewer'}
      <button
        class="text-sm text-indigo-600 hover:underline dark:text-indigo-400"
        onclick={() => {
          showProjectPicker = !showProjectPicker
        }}
      >
        {activeProject ? m.change() : m.assign()}
      </button>
    {/if}
  </div>
  <div class="relative">
    {#if activeProject}
      <span
        class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-sm font-medium"
        style="background-color: {activeProject.color
          ? activeProject.color + '22'
          : '#f3f4f6'}; color: {activeProject.color ?? '#6b7280'}"
      >
        <ColorDot color={activeProject.color ?? '#9ca3af'} size="sm" />
        {activeProject.name}
      </span>
    {:else}
      <span class="text-sm text-gray-400 dark:text-gray-500"
        >{m.not_assigned()}</span
      >
    {/if}

    {#if showProjectPicker}
      <div
        class="absolute top-full left-0 z-20 mt-1 min-w-[180px] rounded-xl border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-900"
      >
        <button
          class="text-md flex w-full items-center gap-2 px-3 py-1.5 text-gray-500 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800"
          onclick={() => assignProject(null)}
        >
          <span class="h-2.5 w-2.5 rounded-full border border-gray-300"></span>
          {m.none()}
        </button>
        {#each projectsStore.projects as p}
          <button
            class="text-md flex w-full items-center gap-2 px-3 py-1.5 text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-800"
            onclick={() => assignProject(p.id)}
          >
            <ColorDot color={p.color ?? '#9ca3af'} size="sm" />
            {p.name}
          </button>
        {/each}
      </div>
    {/if}
  </div>
</div>
