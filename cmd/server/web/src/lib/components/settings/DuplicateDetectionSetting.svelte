<script lang="ts">
  import { workspaceApi } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { m } from '$lib/paraglide/messages'

  type DuplicateMode = 'off' | 'warn' | 'block'

  const isOwner = $derived(authStore.role === 'owner')

  let mode = $state<DuplicateMode>(
    (authStore.workspace?.duplicate_detection_mode as DuplicateMode) ?? 'warn'
  )
  let saving = $state(false)

  $effect(() => {
    mode =
      (authStore.workspace?.duplicate_detection_mode as DuplicateMode) ?? 'warn'
  })

  async function saveMode(next: DuplicateMode) {
    if (!isOwner) return
    // Capture the last confirmed value from the store rather than the local
    // `mode` — by the time this runs, Svelte's own bind:group listener may
    // have already applied the optimistic UI change to `mode`.
    const previous =
      (authStore.workspace?.duplicate_detection_mode as DuplicateMode) ?? 'warn'
    mode = next
    saving = true
    try {
      const updated = await workspaceApi.updateSettings({
        duplicate_detection_mode: next,
      })
      authStore.patchWorkspace({
        duplicate_detection_mode: updated.duplicate_detection_mode,
      })
    } catch (e) {
      mode = previous
      toastStore.show(
        e instanceof Error ? e.message : m.duplicate_settings_save_failed(),
        'error'
      )
    } finally {
      saving = false
    }
  }
</script>

<div
  class="space-y-5 rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-6 shadow-sm"
>
  <div>
    <p class="text-md font-medium text-[var(--text-primary)]">
      {m.duplicate_settings_title()}
    </p>
    <p class="mt-0.5 text-sm text-[var(--text-muted)]">
      {m.duplicate_settings_description()}
    </p>
  </div>

  <label class="flex cursor-pointer items-start gap-3">
    <input
      type="radio"
      name="duplicate_detection_mode"
      class="mt-1.5 accent-indigo-600"
      bind:group={mode}
      value="off"
      disabled={!isOwner || saving}
      onchange={() => saveMode('off')}
    />
    <p class="text-md font-medium text-[var(--text-primary)]">
      {m.duplicate_settings_mode_off()}
    </p>
  </label>

  <label class="flex cursor-pointer items-start gap-3">
    <input
      type="radio"
      name="duplicate_detection_mode"
      class="mt-1.5 accent-indigo-600"
      bind:group={mode}
      value="warn"
      disabled={!isOwner || saving}
      onchange={() => saveMode('warn')}
    />
    <p class="text-md font-medium text-[var(--text-primary)]">
      {m.duplicate_settings_mode_warn()}
    </p>
  </label>

  <label class="flex cursor-pointer items-start gap-3">
    <input
      type="radio"
      name="duplicate_detection_mode"
      class="mt-1.5 accent-indigo-600"
      bind:group={mode}
      value="block"
      disabled={!isOwner || saving}
      onchange={() => saveMode('block')}
    />
    <p class="text-md font-medium text-[var(--text-primary)]">
      {m.duplicate_settings_mode_block()}
    </p>
  </label>
</div>
