<script lang="ts">
  import { untrack } from 'svelte'
  import { type Asset } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { generateDraft } from '$lib/api/drafts'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { m } from '$lib/paraglide/messages'
  import FFmpegCommandInput from './FFmpegCommandInput.svelte'
  import VariantDraftSession from './VariantDraftSession.svelte'

  interface Props {
    asset: Asset
    creating: boolean
    handleCreate: (kind: string, params: Record<string, unknown>) => void
    onDraftStarted?: (nonce: string, meta?: Record<string, unknown>) => void
    sessionActive?: boolean
    initialParams?: Record<string, unknown> | null
  }

  let {
    asset,
    creating,
    handleCreate,
    onDraftStarted,
    sessionActive = false,
    initialParams = null,
  }: Props = $props()

  const kind = 'custom_ffmpeg'

  type Phase = 'form' | 'drafting'
  let phase = $state<Phase>('form')
  let command = $state(untrack(() => (initialParams?.command as string) ?? ''))
  let submitting = $state(false)
  let sessionRef = $state<ReturnType<typeof VariantDraftSession> | undefined>(
    undefined
  )

  function isCommandUsable(cmd: string): boolean {
    const trimmed = cmd.trim()
    if (!trimmed || trimmed.length > 2000) return false
    return (
      (trimmed.match(/\{input\}/g) ?? []).length === 1 &&
      (trimmed.match(/\{output\}/g) ?? []).length === 1
    )
  }

  async function handleTest() {
    if (!isCommandUsable(command)) return
    submitting = true
    try {
      const res = await generateDraft(asset.id, kind, { command })
      if (onDraftStarted) {
        onDraftStarted(res.draft_key, { command })
      } else {
        if (phase !== 'drafting') {
          phase = 'drafting'
          await Promise.resolve()
        }
        sessionRef?.addDraft(res.draft_key, { command })
      }
    } catch (e: unknown) {
      toastStore.show(
        e instanceof Error
          ? e.message
          : m.variants_custom_ffmpeg_error_generic(),
        'error'
      )
    } finally {
      submitting = false
    }
  }

  function handleRunFullFile() {
    if (!isCommandUsable(command)) return
    handleCreate(kind, { command })
  }

  async function keepDraft(
    _nonce: string,
    meta: Record<string, unknown> | undefined
  ): Promise<void> {
    const cmd = (meta?.command as string | undefined) ?? command
    handleCreate(kind, { command: cmd })
  }

  function handleAddMore() {
    phase = 'form'
  }

  $effect(() => {
    if (!sessionActive) phase = 'form'
  })
</script>

{#if phase === 'form' || onDraftStarted}
  <div class="space-y-5">
    <div class="form-header">
      <p class="form-title">{m.variants_custom_ffmpeg_title()}</p>
      <p class="form-desc">
        {m.variants_custom_ffmpeg_description({
          input: '{input}',
          output: '{output}',
        })}
      </p>
    </div>

    <FFmpegCommandInput bind:value={command} disabled={creating} />

    <div class="action-row">
      <Button
        variant="secondary"
        disabled={creating ||
          submitting ||
          authStore.role === 'viewer' ||
          !isCommandUsable(command)}
        onclick={handleTest}
        class="flex-1"
      >
        {submitting ? m.queuing_() : m.variants_custom_ffmpeg_test_button()}
      </Button>
      <Button
        disabled={creating ||
          authStore.role === 'viewer' ||
          !isCommandUsable(command)}
        onclick={handleRunFullFile}
        class="flex-1"
      >
        {creating ? m.queuing_() : m.variants_custom_ffmpeg_submit_button()}
      </Button>
    </div>
  </div>
{:else}
  <VariantDraftSession
    bind:this={sessionRef}
    assetId={asset.id}
    detectMediaKind={true}
    onKeepDraft={keepDraft}
    onDone={handleAddMore}
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
  .form-desc {
    font-size: 0.8125rem;
    color: var(--text-muted);
  }
  .action-row {
    display: flex;
    gap: 8px;
  }
</style>
