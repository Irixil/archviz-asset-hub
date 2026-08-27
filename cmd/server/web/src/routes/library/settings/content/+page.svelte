<script lang="ts">
  import { workspaceApi } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import Modal from '$lib/components/ui/Modal.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import DuplicateDetectionSetting from '$lib/components/settings/DuplicateDetectionSetting.svelte'
  import { m } from '$lib/paraglide/messages'

  type RetentionMode = 'unlimited' | 'capped'

  const isOwner = $derived(authStore.role === 'owner')
  const ws = $derived(authStore.workspace)

  // Version retention
  let mode = $state<RetentionMode>('unlimited')
  let capValue = $state(10)
  let savingRetention = $state(false)

  $effect(() => {
    if (ws) {
      if (ws.version_retention_count > 0) {
        mode = 'capped'
        capValue = ws.version_retention_count
      } else {
        mode = 'unlimited'
      }
    }
  })

  const isRetentionDirty = $derived(
    ws
      ? mode === 'unlimited'
        ? ws.version_retention_count !== 0
        : ws.version_retention_count !== capValue
      : false
  )

  async function saveRetention() {
    savingRetention = true
    try {
      const count =
        mode === 'unlimited' ? 0 : Math.max(1, Math.min(50, capValue))
      const updated = await workspaceApi.updateSettings({
        version_retention_count: count,
      })
      authStore.patchWorkspace({
        version_retention_count: updated.version_retention_count,
      })
      toastStore.show(m.history_settings_saved())
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.history_settings_failed(),
        'error'
      )
    } finally {
      savingRetention = false
    }
  }

  // EXIF privacy
  let exifKeep = $state(false)
  let exifKeepGPS = $state(false)
  let savingExif = $state(false)
  let showBackfillConfirm = $state(false)
  let exifBackfillRunning = $state(false)

  $effect(() => {
    if (ws) {
      exifKeep = ws.exif_keep ?? false
      exifKeepGPS = ws.exif_keep_gps ?? false
    }
  })

  const isExifDirty = $derived(
    ws
      ? exifKeep !== (ws.exif_keep ?? false) ||
          exifKeepGPS !== (ws.exif_keep_gps ?? false)
      : false
  )

  async function saveExif() {
    savingExif = true
    try {
      const updated = await workspaceApi.updateSettings({
        version_retention_count: ws?.version_retention_count ?? 0,
        exif_keep: !!exifKeep,
        exif_keep_gps: !!exifKeepGPS,
      })
      authStore.patchWorkspace({
        exif_keep: !!updated.exif_keep,
        exif_keep_gps: !!updated.exif_keep_gps,
      })
      toastStore.show(m.privacy_settings_saved())
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.privacy_settings_failed(),
        'error'
      )
    } finally {
      savingExif = false
    }
  }

  async function triggerExifBackfill() {
    showBackfillConfirm = false
    exifBackfillRunning = true
    try {
      const result = await workspaceApi.triggerJob('extract_exif')
      if (result.enqueued === 0) {
        toastStore.show('All assets already have EXIF data extracted')
      } else {
        toastStore.show(
          `Queued EXIF extraction for ${result.enqueued} asset${result.enqueued === 1 ? '' : 's'}`
        )
      }
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to queue backfill',
        'error'
      )
    } finally {
      exifBackfillRunning = false
    }
  }

  // Visual similarity backfill
  let visualSimBackfillRunning = $state(false)
  let showVisualSimShowConfirm = $state(false)

  async function triggerBackfill() {
    showVisualSimShowConfirm = false
    visualSimBackfillRunning = true
    try {
      await workspaceApi.triggerJob('visual-similarity-backfill')
      toastStore.show('Visual similarity backfill queued')
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to queue job',
        'error'
      )
    } finally {
      visualSimBackfillRunning = false
    }
  }
</script>

<svelte:head>
  <title>{m.settings_content_title()}｜建筑素材中台</title>
</svelte:head>

<div class="flex flex-1 flex-col overflow-hidden">
  <PageHeader
    title={m.settings_content_title()}
    description={m.settings_content_description()}
  />
  <div class="flex-1 overflow-y-auto px-6 py-6">
    <div class="mx-auto w-full max-w-3xl space-y-8">
      {#if !isOwner}
        <p
          class="text-md rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
        >
          {m.version_history_settins_only_owners()}
        </p>
      {/if}

      <!-- Visual similarity -->
      <div
        class="space-y-6 rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-6 shadow-sm"
      >
        <div class="flex items-start justify-between gap-4">
          <div class="flex-1">
            <p class="text-md font-medium text-[var(--text-primary)]">
              {m.visual_similarity_backfill()}
            </p>
            <p class="mt-0.5 text-sm text-[var(--text-muted)]">
              {m.visual_similarity_backfill_description()}
            </p>
          </div>
        </div>
        <div class="flex justify-start">
          <Button
            variant="secondary"
            disabled={visualSimBackfillRunning || !isOwner}
            loading={visualSimBackfillRunning}
            onclick={() => (showVisualSimShowConfirm = true)}
          >
            {m.visual_similarity_backfill_run()}
          </Button>
        </div>
      </div>

      <!-- Version retention -->
      <div
        class="space-y-5 rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-6 shadow-sm"
      >
        <p class="text-md font-medium text-[var(--text-primary)]">
          {m.version_history()}
        </p>

        <label class="flex cursor-pointer items-start gap-3">
          <input
            type="radio"
            name="retention"
            value="unlimited"
            class="mt-1.5 accent-indigo-600"
            bind:group={mode}
            disabled={!isOwner}
          />
          <div class="flex-1 space-y-1.5">
            <p class="text-md font-medium text-[var(--text-primary)]">
              {m.keep_all_versions()}
            </p>
            <p class="text-sm text-[var(--text-muted)]">
              {m.keep_all_versions_description()}
            </p>
          </div>
        </label>

        <label class="flex cursor-pointer items-start gap-3">
          <input
            type="radio"
            name="retention"
            value="capped"
            class="mt-1.5 accent-indigo-600"
            bind:group={mode}
            disabled={!isOwner}
          />
          <div class="flex-1 space-y-1.5">
            <p class="text-md font-medium text-[var(--text-primary)]">
              {m.keep_n_versions()}
            </p>
            <p class="text-sm text-[var(--text-muted)]">
              {m.keep_n_versions_description()}
            </p>
            {#if mode === 'capped'}
              <div class="flex items-center gap-3 pt-1">
                <input
                  type="number"
                  min="1"
                  max="50"
                  class="text-md w-24 rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-1.5 text-[var(--text-primary)] focus:border-indigo-400 focus:outline-none"
                  bind:value={capValue}
                  disabled={!isOwner}
                />
                <span class="text-sm text-[var(--text-muted)]"
                  >{m.keep_n_versions_ex()}</span
                >
              </div>
            {/if}
          </div>
        </label>

        {#if isOwner}
          <div class="flex justify-end">
            <Button
              variant="primary"
              disabled={!isRetentionDirty}
              loading={savingRetention}
              onclick={saveRetention}
            >
              {m.save()}
            </Button>
          </div>
        {/if}
      </div>

      <!-- Duplicate detection -->
      <DuplicateDetectionSetting />

      <!-- EXIF privacy -->
      <div
        class="space-y-6 rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-6 shadow-sm"
      >
        <p class="text-md font-medium text-[var(--text-primary)]">
          {m.tab_exif_privacy()}
        </p>

        <div class="flex items-start justify-between gap-4">
          <div class="flex-1">
            <p class="text-md font-medium text-[var(--text-primary)]">
              {m.extract_exif()}
            </p>
            <p class="mt-0.5 text-sm text-[var(--text-muted)]">
              {m.extract_exif_description()}
            </p>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={exifKeep}
            aria-label={m.extract_exif()}
            disabled={!isOwner}
            onclick={() => {
              if (isOwner) exifKeep = !exifKeep
            }}
            class="relative mt-0.5 inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus:outline-none disabled:cursor-not-allowed disabled:opacity-50
            {exifKeep ? 'bg-indigo-600' : 'bg-gray-200 dark:bg-gray-700'}"
          >
            <span
              class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow ring-0 transition-transform
            {exifKeep ? 'translate-x-5' : 'translate-x-0'}"
            ></span>
          </button>
        </div>

        {#if exifKeep}
          <div
            class="flex items-start justify-between gap-4 border-t border-[var(--border-subtle)] pt-6"
          >
            <div class="flex-1">
              <p class="text-md font-medium text-[var(--text-primary)]">
                {m.keep_gps()}
              </p>
              <p class="mt-0.5 text-sm text-[var(--text-muted)]">
                {m.keep_gps_description()}
              </p>
            </div>
            <button
              type="button"
              role="switch"
              aria-checked={exifKeepGPS}
              aria-label={m.keep_gps()}
              disabled={!isOwner}
              onclick={() => {
                if (isOwner) exifKeepGPS = !exifKeepGPS
              }}
              class="relative mt-0.5 inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus:outline-none disabled:cursor-not-allowed disabled:opacity-50
              {exifKeepGPS ? 'bg-indigo-600' : 'bg-gray-200 dark:bg-gray-700'}"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow ring-0 transition-transform
              {exifKeepGPS ? 'translate-x-5' : 'translate-x-0'}"
              ></span>
            </button>
          </div>
        {/if}

        {#if isOwner}
          <div class="flex items-center justify-between">
            <Button
              variant="secondary"
              disabled={exifBackfillRunning || !exifKeep}
              loading={exifBackfillRunning}
              title={!exifKeep ? m.extract_exif_first() : undefined}
              onclick={() => (showBackfillConfirm = true)}
            >
              {m.extract_exif_again()}
            </Button>

            <Button
              variant="primary"
              disabled={!isExifDirty}
              loading={savingExif}
              onclick={saveExif}
            >
              {m.save()}
            </Button>
          </div>
        {/if}
      </div>
    </div>
  </div>
</div>

<Modal
  bind:open={showVisualSimShowConfirm}
  onclose={() => (showVisualSimShowConfirm = false)}
>
  <div class="p-6">
    <h2 class="mb-3 text-lg font-semibold text-[var(--text-primary)]">
      {m.visual_similarity_backfill()}?
    </h2>
    <p class="text-md text-[var(--text-secondary)]">
      {m.visual_similarity_backfill_description()}
    </p>
    <div class="mt-6 flex justify-end gap-3">
      <Button
        variant="secondary"
        onclick={() => (showVisualSimShowConfirm = false)}
      >
        {m.cancel()}
      </Button>
      <Button variant="primary" onclick={triggerBackfill}>
        {m.visual_similarity_backfill_run()}
      </Button>
    </div>
  </div>
</Modal>

{#if showBackfillConfirm}
  <Modal onclose={() => (showBackfillConfirm = false)}>
    <h2 class="mb-3 text-lg font-semibold text-[var(--text-primary)]">
      {m.extract_exif_again()}?
    </h2>
    <p class="text-md text-[var(--text-secondary)]">
      {m.extract_exif_again_description()}
    </p>
    <div class="mt-6 flex justify-end gap-3">
      <Button variant="secondary" onclick={() => (showBackfillConfirm = false)}>
        {m.cancel()}
      </Button>
      <Button variant="primary" onclick={triggerExifBackfill}>
        {m.reextract()}
      </Button>
    </div>
  </Modal>
{/if}
