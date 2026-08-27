<script lang="ts">
  import { onMount } from 'svelte'
  import { sharesStore } from '$lib/stores/shares.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import type { Share } from '$lib/api'
  import ShareModal from '$lib/components/ShareModal.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import {
    Eye,
    Globe,
    Link,
    Lock,
    Plus,
    MessageSquare,
    Download,
  } from '@lucide/svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import GridSkeleton from '$lib/components/ui/GridSkeleton.svelte'
  import ButtonDelete from '$lib/components/ui/ButtonDelete.svelte'
  import StatusBadge from '$lib/components/ui/StatusBadge.svelte'
  import ButtonCopy from '$lib/components/ui/ButtonCopy.svelte'
  import PageContainer from '$lib/components/ui/PageContainer.svelte'
  import { m } from '$lib/paraglide/messages'

  let showCreateModal = $state(false)
  let copied = $state<string | null>(null)

  onMount(() => {
    sharesStore.load()
    projectsStore.load()
  })

  function statusInfo(share: Share): { label: string; cls: string } {
    if (share.revoked_at)
      return {
        label: m.revoked(),
        cls: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
      }
    if (share.is_expired)
      return {
        label: m.expired(),
        cls: 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400',
      }
    if (share.expires_at) {
      const days = Math.ceil(
        (new Date(share.expires_at).getTime() - Date.now()) / 86_400_000
      )
      if (days <= 3)
        return {
          label: `Expires in ${days}d`,
          cls: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
        }
      return {
        label: `Expires in ${days}d`,
        cls: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
      }
    }
    return {
      label: m.active(),
      cls: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
    }
  }

  function targetLabel(share: Share): string {
    if (share.target_type === 'project') {
      const p = projectsStore.projects.find((x) => x.id === share.target_id)
      return p
        ? m.project_id({ name: p.name })
        : m.project_id({ name: `${share.target_id.slice(0, 8)}…)` })
    }
    if (share.target_type === 'asset')
      return m.asset_id({ id: `(${share.target_id.slice(0, 8)}…)` })
    return m.collection_id({
      id: `Collection (${share.target_id.slice(0, 8)}…)`,
    })
  }

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString()
  }

  function copyLink(share: Share) {
    navigator.clipboard
      .writeText(share.public_url)
      .then(() => {
        copied = share.id
        toastStore.show(m.link_copied())
        setTimeout(() => {
          copied = null
        }, 2000)
      })
      .catch(() => toastStore.show(m.cannot_copy_link(), 'error'))
  }

  // Default targets for the "New Share" button on this page:
  // show all projects as options
  const createTargets = $derived(
    projectsStore.projects.map((p) => ({
      type: 'project' as const,
      id: p.id,
      label: p.name,
      assetCount: p.asset_count,
    }))
  )
</script>

<svelte:head>
  <title>{m.title_share_links()}｜建筑素材中台</title>
</svelte:head>

<PageContainer>
  <PageHeader
    title={m.title_share_links()}
    description={m.subtitle_shares_description()}
  >
    <Button
      variant="primary"
      disabled={createTargets.length === 0}
      onclick={() => {
        showCreateModal = true
      }}
    >
      {#snippet icon()}<Plus class="h-4 w-4" />{/snippet}
      {m.new_share()}
    </Button>
  </PageHeader>

  <!-- Content -->
  <main class="flex-1 overflow-y-auto px-6 py-6">
    {#if sharesStore.loading}
      <GridSkeleton lines={4} />
    {:else if sharesStore.shares.length === 0}
      <div class="flex flex-col items-center justify-center py-24 text-center">
        <div
          class="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-gray-100 dark:bg-gray-800"
        >
          <Link class="h-8 w-8 text-gray-400 dark:text-gray-500" />
        </div>
        <h2
          class="mb-1 text-base font-semibold text-gray-900 dark:text-gray-50"
        >
          {m.no_share_links()}
        </h2>
        <p class="text-md mb-6 text-gray-400">{m.create_share_description()}</p>
        <Button
          variant="primary"
          onclick={() => {
            showCreateModal = true
          }}
          disabled={createTargets.length === 0}
        >
          {#snippet icon()}<Plus class="h-4 w-4" />{/snippet}
          {m.create_first_shae()}
        </Button>
      </div>
    {:else}
      <div class="space-y-3">
        {#each sharesStore.shares as share (share.id)}
          {@const status = statusInfo(share)}
          <div
            class="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-800 dark:bg-gray-900"
          >
            <div class="flex items-start gap-4 px-5 py-4">
              <!-- Icon -->
              <div
                class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-gray-100 dark:bg-gray-800"
              >
                <Globe class="h-4 w-4 text-gray-500 dark:text-gray-400" />
              </div>

              <!-- Meta -->
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span
                    class="text-md font-medium text-gray-900 dark:text-gray-50"
                  >
                    {share.label || targetLabel(share)}
                  </span>
                  <span
                    class="rounded-full px-2 py-0.5 text-[11px] font-medium {status.cls}"
                    >{status.label}</span
                  >
                  {#if share.has_password}
                    <span
                      class="inline-flex items-center gap-1 text-sm text-amber-600 dark:text-amber-400"
                    >
                      <Lock class="h-3 w-3" />
                      {m.password()}
                    </span>
                  {/if}
                </div>
                <p class="mt-0.5 text-sm text-gray-400 dark:text-gray-500">
                  {targetLabel(share)}
                </p>
                <div
                  class="mt-2 flex flex-wrap items-center gap-3 text-sm text-gray-400 dark:text-gray-500"
                >
                  <span class="inline-flex items-center gap-1">
                    <Eye class="h-3 w-3" />
                    {share.view_count} view{share.view_count !== 1 ? 's' : ''}
                  </span>
                  <span>{m.created_at()} {formatDate(share.created_at)}</span>
                  {#if share.allow_download}
                    <span
                      class="inline-flex items-center gap-1 text-green-600 dark:text-green-500"
                    >
                      <Download class="h-3 w-3" />
                      {m.downloads()}
                    </span>
                  {/if}
                  {#if share.allow_comments}
                    <span
                      class="inline-flex items-center gap-1 text-blue-600 dark:text-blue-500"
                    >
                      <MessageSquare class="h-3 w-3" />
                      {m.comments()}
                    </span>
                  {/if}
                </div>
              </div>

              <!-- Actions -->
              <div class="flex shrink-0 items-center gap-1.5">
                {#if !share.revoked_at}
                  <ButtonCopy
                    onclick={() => copyLink(share)}
                    copied={copied === share.id}
                    title={m.copy_link()}
                  />
                  <ButtonDelete
                    onclick={() => sharesStore.revoke(share.id)}
                    title={m.revoke()}
                  />
                {:else}
                  <StatusBadge i status="disabled" text={m.revoked()} />
                {/if}
              </div>
            </div>

            <!-- URL bar for active shares -->
            {#if !share.revoked_at && !share.is_expired}
              <div
                class="border-t border-gray-50 bg-gray-50/60 px-5 py-2.5 dark:border-gray-800 dark:bg-gray-800/40"
              >
                <span class="font-mono text-sm text-gray-400 dark:text-gray-500"
                  >{share.public_url}</span
                >
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </main>
</PageContainer>

{#if showCreateModal && createTargets.length > 0}
  <ShareModal
    bind:open={showCreateModal}
    targets={createTargets}
    onclose={() => {
      showCreateModal = false
    }}
  />
{/if}
