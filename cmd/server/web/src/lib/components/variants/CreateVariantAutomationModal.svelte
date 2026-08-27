<script lang="ts">
  import { goto } from '$app/navigation'
  import type { Variant } from '$lib/api'
  import { variantApi } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import Modal from '$lib/components/ui/Modal.svelte'

  type Scope = 'workspace' | 'project' | 'folder' | 'asset'

  let {
    assetId,
    assetProjectId,
    assetFolderId,
    assetVariants,
    onClose,
  }: {
    assetId: string
    assetProjectId: string | null
    assetFolderId: string | null
    assetVariants: Variant[]
    onClose: () => void
  } = $props()

  // svelte-ignore state_referenced_locally
  let scope = $state<Scope>('asset')
  let loading = $state(false)
  let error = $state<string | null>(null)

  const scopeOptions = $derived([
    {
      value: 'workspace' as Scope,
      label: m.variant_automation_scope_workspace(),
    },
    ...(assetProjectId
      ? [
          {
            value: 'project' as Scope,
            label: m.variant_automation_scope_project(),
          },
        ]
      : []),
    ...(assetFolderId
      ? [
          {
            value: 'folder' as Scope,
            label: m.variant_automation_scope_folder(),
          },
        ]
      : []),
    ...[
      {
        value: 'asset' as Scope,
        label: m.variant_automation_scope_asset(),
      },
    ],
  ])

  async function submit() {
    loading = true
    error = null
    try {
      const result = await variantApi.automate(assetId, scope)
      onClose()
      await goto(result.workflow_url)
    } catch {
      error = m.variant_automation_error_create()
    } finally {
      loading = false
    }
  }
</script>

<Modal open={true} onclose={onClose}>
  <div class="space-y-5 p-5">
    <header class="flex items-center justify-between gap-3">
      <h2 class="text-lg font-semibold text-[var(--text-primary)]">
        {m.variant_automation_modal_title()}
      </h2>
      <button
        type="button"
        class="rounded-md px-2 py-1 text-xl leading-none text-[var(--text-muted)] hover:bg-[var(--bg-hover)]"
        aria-label={m.cancel()}
        onclick={onClose}
      >
        ×
      </button>
    </header>

    <section class="space-y-3">
      <p class="text-sm text-[var(--text-muted)]">
        {m.variant_automation_preview_hint()}
      </p>
      <div class="flex flex-wrap gap-2">
        {#each assetVariants as variant}
          <span
            class="rounded-md border border-[var(--border)] bg-[var(--bg-secondary)] px-2 py-1 text-xs font-medium text-[var(--text-secondary)]"
          >
            {variant.type}
          </span>
        {/each}
      </div>
    </section>

    <section class="space-y-2">
      <p class="text-sm font-medium text-[var(--text-primary)]">
        {m.variant_automation_scope_label()}
      </p>
      <div class="grid gap-2">
        {#each scopeOptions as opt}
          <label
            class="flex cursor-pointer items-center gap-3 rounded-lg border border-[var(--border)] px-3 py-2 text-sm text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-hover)]"
            class:selected={scope === opt.value}
          >
            <input
              type="radio"
              name="scope"
              value={opt.value}
              bind:group={scope}
            />
            <span>{opt.label}</span>
          </label>
        {/each}
      </div>
    </section>

    {#if error}
      <p
        class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300"
      >
        {error}
      </p>
    {/if}

    <footer class="flex justify-end gap-2">
      <button
        type="button"
        class="rounded-lg border border-[var(--border)] px-4 py-2 text-sm font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]"
        onclick={onClose}
      >
        {m.cancel()}
      </button>
      <button
        type="button"
        class="rounded-lg bg-[var(--color-primary)] px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
        disabled={loading}
        onclick={submit}
      >
        {loading ? m.creating_account() : m.variant_automation_create_btn()}
      </button>
    </footer>
  </div>
</Modal>

<style>
  .selected {
    border-color: var(--color-primary);
    background: color-mix(in oklab, var(--color-primary) 10%, transparent);
  }
</style>
