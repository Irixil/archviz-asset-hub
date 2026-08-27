<script lang="ts">
  import { embedTokenApi, type EmbedTokenResponse } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { Check, Link, Loader2, Trash2 } from '@lucide/svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    assetId: string
  }

  let { assetId }: Props = $props()

  let token = $state<EmbedTokenResponse | null>(null)
  let loading = $state(true)
  let copied = $state(false)
  let confirmingRevoke = $state(false)
  let copiedTimer: ReturnType<typeof setTimeout> | undefined

  const canManage = $derived(authStore.role !== 'viewer')

  $effect(() => {
    token = null
    confirmingRevoke = false
    loading = true
    embedTokenApi
      .get(assetId)
      .then((t) => {
        token = t
      })
      .finally(() => {
        loading = false
      })
  })

  function copyToClipboard(url: string) {
    navigator.clipboard
      .writeText(url)
      .then(() => {
        copied = true
        toastStore.show(m.link_copied())
        clearTimeout(copiedTimer)
        copiedTimer = setTimeout(() => {
          copied = false
        }, 2000)
      })
      .catch(() => {
        toastStore.show(m.embed_copy_failed(), 'error')
      })
  }

  async function handleClick() {
    if (token) {
      copyToClipboard(token.public_url)
      return
    }
    loading = true
    try {
      token = await embedTokenApi.create(assetId)
      copyToClipboard(token.public_url)
    } catch {
      toastStore.show(m.embed_create_failed(), 'error')
    } finally {
      loading = false
    }
  }

  async function handleRevoke() {
    loading = true
    try {
      await embedTokenApi.revoke(assetId)
      token = null
      confirmingRevoke = false
      toastStore.show(m.embed_revoke_success())
    } catch {
      toastStore.show(m.embed_revoke_failed(), 'error')
    } finally {
      loading = false
    }
  }
</script>

{#if loading || token || canManage}
  <div class="space-y-2">
    <div class="flex items-center gap-2">
      <button
        class="flex flex-1 items-center gap-3 rounded-lg border border-[var(--border)] px-3 py-2 text-sm font-medium whitespace-nowrap text-[var(--text-primary)] transition hover:bg-[var(--bg-hover)] disabled:opacity-50"
        onclick={handleClick}
        disabled={loading}
      >
        {#if loading}
          <Loader2
            class="h-4 w-4 shrink-0 animate-spin text-[var(--text-muted)]"
          />
        {:else if copied}
          <Check class="h-4 w-4 shrink-0 text-[var(--text-muted)]" />
        {:else}
          <Link class="h-4 w-4 shrink-0 text-[var(--text-muted)]" />
        {/if}
        <span class="flex-1 text-left">
          {#if loading}
            {m.loading()}
          {:else if token}
            {m.embed_public_url_active()}
          {:else}
            {m.embed_copy_public_url()}
          {/if}
        </span>
      </button>

      {#if token && canManage}
        <button
          class="shrink-0 rounded-lg border border-[var(--border)] p-2 text-[var(--text-muted)] transition hover:bg-[var(--bg-hover)] hover:text-red-600 disabled:opacity-50"
          onclick={() => {
            confirmingRevoke = true
          }}
          disabled={loading}
          aria-label={m.embed_revoke()}
        >
          <Trash2 class="h-4 w-4" />
        </button>
      {/if}
    </div>

    {#if confirmingRevoke}
      <div
        class="rounded-lg border border-[var(--border)] bg-[var(--bg-hover)] p-3 text-sm"
      >
        <p class="mb-1 font-medium text-[var(--text-primary)]">
          {m.embed_revoke_confirm_title()}
        </p>
        <p class="mb-3 text-[var(--text-muted)]">
          {m.embed_revoke_confirm_body()}
        </p>
        <div class="flex gap-2">
          <button
            class="flex-1 rounded-lg border border-[var(--border)] px-3 py-1.5 text-sm font-medium text-[var(--text-primary)] transition hover:bg-[var(--bg-hover)]"
            onclick={() => {
              confirmingRevoke = false
            }}
          >
            {m.cancel()}
          </button>
          <button
            class="flex-1 rounded-lg bg-red-600 px-3 py-1.5 text-sm font-medium text-white transition hover:bg-red-700 disabled:opacity-50"
            onclick={handleRevoke}
            disabled={loading}
          >
            {m.embed_revoke()}
          </button>
        </div>
      </div>
    {/if}
  </div>
{/if}
