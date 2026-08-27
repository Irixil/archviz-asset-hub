<script lang="ts">
  import {
    versionApi,
    formatBytes,
    type Asset,
    type AssetVersion,
  } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { Clock, Download, RotateCcw, Inbox } from '@lucide/svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import Modal from '$lib/components/ui/Modal.svelte'
  import Feedback from './ui/Feedback.svelte'
  import ButtonDelete from './ui/ButtonDelete.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    asset: Asset
    onVersionChanged: (updatedAsset: Asset) => void
  }

  let { asset, onVersionChanged }: Props = $props()

  let versions = $state<AssetVersion[]>([])
  let loading = $state(false)
  let error = $state('')

  // Restore confirmation state
  let restoreTarget = $state<AssetVersion | null>(null)
  let restoring = $state(false)
  let restoreError = $state('')
  let showRestoreModal = $derived(restoreTarget !== null)

  // Delete confirmation state
  let deleteTarget = $state<AssetVersion | null>(null)
  let deleting = $state(false)
  let deleteError = $state('')
  let showDeleteModal = $derived(deleteTarget !== null)

  $effect(() => {
    if (asset) loadVersions()
  })

  async function loadVersions() {
    loading = true
    error = ''
    try {
      versions = await versionApi.list(asset.id)
    } catch (e) {
      error = e instanceof Error ? e.message : m.versions_load_failed()
    } finally {
      loading = false
    }
  }

  async function confirmRestore() {
    if (!restoreTarget) return
    restoring = true
    restoreError = ''
    try {
      const res = await versionApi.restore(asset.id, restoreTarget.id)
      versions = await versionApi.list(asset.id)
      onVersionChanged(res.asset)
      restoreTarget = null
    } catch (e) {
      restoreError = e instanceof Error ? e.message : m.version_restore_failed()
    } finally {
      restoring = false
    }
  }

  async function confirmDelete() {
    if (!deleteTarget) return
    deleting = true
    deleteError = ''
    try {
      await versionApi.delete(asset.id, deleteTarget.id)
      versions = versions.filter((v) => v.id !== deleteTarget!.id)
      deleteTarget = null
    } catch (e) {
      deleteError = e instanceof Error ? e.message : m.version_delete_failed()
    } finally {
      deleting = false
    }
  }

  function relativeTime(iso: string): string {
    const diff = Date.now() - new Date(iso).getTime()
    const mins = Math.floor(diff / 60000)
    if (mins < 1) return 'just now'
    if (mins < 60) return `${mins}m ago`
    const hrs = Math.floor(mins / 60)
    if (hrs < 24) return `${hrs}h ago`
    const days = Math.floor(hrs / 24)
    if (days < 30) return `${days}d ago`
    return new Date(iso).toLocaleDateString()
  }

  function formatDate(iso: string): string {
    return new Date(iso).toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
    })
  }
</script>

<div class="flex flex-col">
  {#if loading}
    <div class="flex justify-center py-12">
      <Spinner size="md" />
    </div>
  {:else if error}
    <Feedback {error} />
  {:else if versions.length === 0}
    <div
      class="flex flex-col items-center gap-3 py-12 text-center text-gray-400"
    >
      <Inbox class="h-10 w-10" />
      <p class="text-md">{m.no_versions_yet()}</p>
    </div>
  {:else}
    <ol
      class="relative mx-5 mt-4 mb-2 border-l border-gray-200 dark:border-gray-700"
    >
      {#each versions as v (v.id)}
        <li class="mb-6 ml-4">
          <!-- Timeline dot -->
          <div
            class="absolute -left-1.5 mt-1.5 h-3 w-3 rounded-full border-2 border-white bg-gray-300 dark:border-gray-900 dark:bg-gray-600
            {v.is_current ? '!bg-indigo-500 dark:!bg-indigo-400' : ''}"
          ></div>

          <div
            class="rounded-xl border border-gray-100 bg-gray-50 p-3 dark:border-gray-700 dark:bg-gray-800"
          >
            <!-- Header row -->
            <div class="flex items-start justify-between gap-2">
              <div class="flex min-w-0 items-center gap-2">
                <!-- Version badge -->
                <span
                  class="shrink-0 rounded-full px-2 py-0.5 text-[11px] font-bold
                  {v.is_current
                    ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/50 dark:text-indigo-300'
                    : 'bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-300'}"
                >
                  v{v.version_num}
                </span>
                {#if v.is_current}
                  <span
                    class="rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-semibold text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-400"
                  >
                    {m.current_version()}
                  </span>
                {/if}
              </div>
              <!-- Timestamp -->
              <span
                class="flex shrink-0 items-center gap-1 text-[11px] text-gray-400 dark:text-gray-500"
              >
                <Clock class="h-3 w-3" />
                {relativeTime(v.created_at)}
              </span>
            </div>

            <!-- Thumbnail + meta -->
            <div class="mt-2 flex gap-3">
              {#if v.thumbnail_url}
                <img
                  src={v.thumbnail_url}
                  alt="v{v.version_num} thumbnail"
                  class="h-12 w-12 shrink-0 rounded-lg object-cover"
                  onerror={(e) => {
                    ;(e.currentTarget as HTMLImageElement).style.display =
                      'none'
                  }}
                />
              {:else}
                <div
                  class="h-12 w-12 shrink-0 rounded-lg bg-gray-200 dark:bg-gray-700"
                ></div>
              {/if}
              <div class="flex min-w-0 flex-col gap-0.5">
                <div class="flex flex-wrap items-center gap-2">
                  <p class="text-sm text-gray-600 dark:text-gray-400">
                    {formatBytes(v.size)}
                    {#if v.width && v.height}
                      · {v.width}×{v.height}
                    {/if}
                  </p>
                  <!-- Variant count chip (VV-4.2) -->
                  {#if v.variant_count > 0}
                    <span
                      class="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500 dark:bg-gray-700 dark:text-gray-400"
                    >
                      {v.variant_count} variant{v.variant_count === 1
                        ? ''
                        : 's'}
                    </span>
                  {/if}
                </div>
                {#if v.comment}
                  <p
                    class="line-clamp-2 text-sm text-gray-500 italic dark:text-gray-400"
                  >
                    "{v.comment}"
                  </p>
                {/if}
                <p class="text-[11px] text-gray-400 dark:text-gray-500">
                  {m.by_name({ name: v?.created_by?.name || 'Unknown' })} · {formatDate(
                    v.created_at
                  )}
                </p>
              </div>
            </div>

            <!-- Actions -->
            <div class="mt-2.5 flex flex-wrap items-center gap-1.5">
              <a
                href={versionApi.fileUrl(asset.id, v.id)}
                download
                class="inline-flex items-center gap-1 rounded-lg border border-gray-200 px-2.5 py-1 text-sm text-gray-600 transition-colors hover:bg-gray-100 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
              >
                <Download class="h-3 w-3" />
                {m.download()}
              </a>

              {#if !v.is_current && authStore.role !== 'viewer'}
                <button
                  type="button"
                  class="inline-flex items-center gap-1 rounded-lg border border-indigo-200 px-2.5 py-1 text-sm text-indigo-600 transition-colors hover:bg-indigo-50 dark:border-indigo-700 dark:text-indigo-400 dark:hover:bg-indigo-900/30"
                  onclick={() => {
                    restoreTarget = v
                    restoreError = ''
                  }}
                >
                  <RotateCcw class="h-3 w-3" />
                  {m.restore()}
                </button>
              {/if}

              {#if !v.is_current && authStore.role === 'owner'}
                <ButtonDelete
                  onclick={() => {
                    deleteTarget = v
                    deleteError = ''
                  }}
                />
              {/if}
            </div>
          </div>
        </li>
      {/each}
    </ol>
  {/if}
</div>

<!-- Restore confirmation modal -->
<Modal
  bind:open={showRestoreModal}
  onclose={() => {
    restoreTarget = null
    restoreError = ''
  }}
>
  {#if restoreTarget}
    <div class="space-y-4 p-6">
      <h3 class="text-xl font-semibold text-gray-900 dark:text-gray-100">
        {m.version_restore_id({ id: restoreTarget.version_num })}
      </h3>
      <div class="text-md space-y-2 text-gray-600 dark:text-gray-300">
        <p>
          {m.uploaded_by_on({
            name: restoreTarget?.created_by?.name || 'Unknown',
            date: formatDate(restoreTarget.created_at),
          })}
        </p>
        {#if restoreTarget.comment}
          <p class="italic">"{restoreTarget.comment}"</p>
        {/if}
        <p class="text-gray-500 dark:text-gray-400">
          {m.current_version_kept_in_history()}
        </p>
        <Feedback error={restoreError} />
      </div>
      <div class="flex justify-end gap-3 pt-2">
        <button
          type="button"
          class="text-md rounded-lg border border-gray-200 px-4 py-2 text-gray-600 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
          onclick={() => {
            restoreTarget = null
            restoreError = ''
          }}
        >
          {m.cancel()}
        </button>
        <button
          type="button"
          class="text-md flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 font-medium text-white hover:bg-indigo-700 disabled:opacity-60"
          disabled={restoring}
          onclick={confirmRestore}
        >
          {#if restoring}<Spinner size="sm" />{/if}
          {m.version_restore_id({ id: restoreTarget.version_num })}
        </button>
      </div>
    </div>
  {/if}
</Modal>

<!-- Delete confirmation modal -->
<Modal
  bind:open={showDeleteModal}
  onclose={() => {
    deleteTarget = null
    deleteError = ''
  }}
>
  {#if deleteTarget}
    <div class="space-y-4 p-6">
      <h3 class="text-xl font-semibold text-gray-900 dark:text-gray-100">
        {m.version_delete_id({ id: deleteTarget.version_num })}
      </h3>
      <div class="text-md space-y-2 text-gray-600 dark:text-gray-300">
        <p>{m.version_delete_grace_period()}</p>
        <Feedback error={deleteError} />
      </div>
      <div class="flex justify-end gap-3 pt-2">
        <button
          type="button"
          class="text-md rounded-lg border border-gray-200 px-4 py-2 text-gray-600 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
          onclick={() => {
            deleteTarget = null
            deleteError = ''
          }}
        >
          {m.cancel()}
        </button>
        <button
          type="button"
          class="text-md flex items-center gap-2 rounded-lg bg-red-600 px-4 py-2 font-medium text-white hover:bg-red-700 disabled:opacity-60"
          disabled={deleting}
          onclick={confirmDelete}
        >
          {#if deleting}<Spinner size="sm" />{/if}
          {m.delete()}
        </button>
      </div>
    </div>
  {/if}
</Modal>
