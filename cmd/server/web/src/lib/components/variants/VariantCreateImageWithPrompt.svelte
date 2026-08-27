<script lang="ts">
  import { untrack } from 'svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { type Asset } from '$lib/api'
  import { generateDraft } from '$lib/api/drafts'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { m } from '$lib/paraglide/messages'
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

  const kind = 'image_with_prompt'

  let phase = $state<Phase>('form')
  let prompt = $state(untrack(() => (initialParams?.prompt as string) ?? ''))
  let model = $state(untrack(() => (initialParams?.model as string) ?? ''))
  let provider = $state<ProviderId>(
    untrack(() => (initialParams?.provider as ProviderId) ?? 'imagerouter')
  )
  let modelsMulti = $state<string[]>([])
  let defaultModelId = $state('')
  let configured = $state(true)
  let submitting = $state(false)
  let sessionRef = $state<ReturnType<typeof VariantDraftSession> | undefined>(
    undefined
  )

  function handleModelsLoaded(res: AIProvider | undefined) {
    configured = !!res?.configured
    // defaultModelId = res?.default_bg_remove_model
    if (!model) model = res?.models.find(Boolean)?.name ?? ''
  }

  async function handlePreview() {
    submitting = true
    try {
      const targets = modelsMulti.length > 0 ? modelsMulti : [model]
      for (const target of targets) {
        const res = await generateDraft(asset.id, kind, {
          prompt: prompt.trim(),
          provider: provider,
          model: target,
        })
        if (onDraftStarted) {
          onDraftStarted(res.draft_key)
        } else {
          if (phase !== 'drafting') {
            phase = 'drafting'
            await Promise.resolve()
          }
          sessionRef?.addDraft(res.draft_key)
        }
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
    <div class="form-header">
      <p class="form-title">{m.image_with_prompt_title()}</p>
      <p class="form-desc">{m.image_with_prompt_description()}</p>
    </div>

    <div>
      <label for="variant-{kind}-prompt" class="field-label">
        {m.image_with_prompt_prompt_label()}
      </label>
      <textarea
        id="variant-{kind}-prompt"
        bind:value={prompt}
        rows="4"
        class="field-input textarea"
        placeholder={m.image_with_prompt_prompt_placeholder()}
      ></textarea>
    </div>

    <AIModelSelect
      bind:value={model}
      bind:selectedProviderName={provider}
      bind:values={modelsMulti}
      {defaultModelId}
      disabled={submitting}
      allowMulti={true}
      onloaded={handleModelsLoaded}
    />

    <p class="form-note">{m.image_with_prompt_output_note()}</p>

    <Button
      disabled={submitting ||
        authStore.role === 'viewer' ||
        !configured ||
        !prompt.trim() ||
        (modelsMulti.length === 0 && !model)}
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
  .form-header {
    padding-bottom: 4px;
    border-bottom: 1px solid var(--border-subtle);
  }
  .form-title {
    font-size: 0.9375rem;
    font-weight: 600;
    color: var(--text-primary);
  }
  .form-desc,
  .form-note {
    font-size: 0.8125rem;
    color: var(--text-muted);
  }
  .field-label {
    display: block;
    margin-bottom: 4px;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-secondary);
  }
  .field-input {
    width: 100%;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    color: var(--text-primary);
    padding: 7px 10px;
    font-size: 0.875rem;
    outline: none;
    transition: border-color 0.12s ease;
  }
  .field-input:focus {
    border-color: var(--accent-cta);
  }
  .textarea {
    min-height: 92px;
    resize: vertical;
  }
</style>
