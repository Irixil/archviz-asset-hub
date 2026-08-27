<script lang="ts">
  import { untrack } from 'svelte'
  import { m } from '$lib/paraglide/messages'
  import { type Asset } from '$lib/api'
  import { generateDraft } from '$lib/api/drafts'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import AIModelSelect from './AIModelSelect.svelte'
  import VariantDraftSession from './VariantDraftSession.svelte'
  import type { AIProvider, ProviderId } from '$lib/api/ai_providers'

  interface Props {
    asset: Asset
    onDone?: () => void
    onDraftStarted?: (nonce: string) => void
    sessionActive?: boolean
    initialParams?: Record<string, unknown> | null
  }

  let {
    asset,
    onDone,
    onDraftStarted,
    sessionActive = false,
    initialParams = null,
  }: Props = $props()

  type Phase = 'form' | 'drafting'

  const kind = 'image_bg_remove'
  let phase = $state<Phase>('form')
  let model = $state(untrack(() => (initialParams?.model as string) ?? ''))
  let provider = $state<ProviderId>(
    untrack(() => (initialParams?.provider as ProviderId) ?? 'imagerouter')
  )
  let defaultModelId = $state('')
  let configured = $state(true)
  let submitting = $state(false)
  let sessionRef = $state<ReturnType<typeof VariantDraftSession> | undefined>(
    undefined
  )

  function handleModelsLoaded(res: AIProvider | undefined) {
    configured = !!res?.configured
    // defaultModelId = res?.default_bg_remove_model
    if (!model) model = res?.models.find((m) => m.name.match(/bg/))?.name ?? ''
  }

  async function handlePreview() {
    submitting = true
    try {
      const res = await generateDraft(asset.id, kind, { model, provider })
      if (onDraftStarted) {
        onDraftStarted(res.draft_key)
      } else {
        phase = 'drafting'
        await Promise.resolve()
        sessionRef?.addDraft(res.draft_key)
      }
    } catch (e: unknown) {
      toastStore.show(
        e instanceof Error ? e.message : m.variant_create_failed(),
        'error'
      )
    } finally {
      submitting = false
    }
  }

  function handleAddMore() {
    phase = 'form'
  }

  function handleDone() {
    onDone?.()
  }

  $effect(() => {
    if (!sessionActive) phase = 'form'
  })
</script>

{#if phase === 'form' || onDraftStarted}
  <div class="space-y-5">
    <div class="notice">
      <p class="notice-title">{m.bg_remove()}</p>
      <p class="notice-body">{m.image_bg_remove_description()}</p>
    </div>

    <AIModelSelect
      bind:selectedProviderName={provider}
      bind:value={model}
      {defaultModelId}
      disabled={submitting}
      onloaded={handleModelsLoaded}
    />

    <Button
      disabled={submitting || authStore.role === 'viewer' || !configured}
      onclick={handlePreview}
      class="w-full"
    >
      {submitting ? m.queuing_() : m.variants_draft_preview_button()}
    </Button>
  </div>
{:else}
  <VariantDraftSession
    bind:this={sessionRef}
    assetId={asset.id}
    onDone={handleDone}
    onAddMore={handleAddMore}
    onRestoreSession={() => {
      phase = 'drafting'
    }}
  />
{/if}

<style>
  .notice {
    border-radius: 8px;
    border: 1px solid oklch(85% 0.08 80);
    background: oklch(97% 0.03 80);
    padding: 12px 14px;
  }
  :global(.dark) .notice {
    border-color: oklch(40% 0.08 80 / 0.5);
    background: oklch(30% 0.06 80 / 0.2);
  }
  .notice-title {
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--text-primary);
    margin-bottom: 4px;
  }
  :global(.dark) .notice-title {
    color: var(--text-primary);
  }
  .notice-body {
    font-size: 0.8125rem;
    color: var(--text-muted);
  }
  :global(.dark) .notice-body {
    color: var(--text-muted);
  }
</style>
