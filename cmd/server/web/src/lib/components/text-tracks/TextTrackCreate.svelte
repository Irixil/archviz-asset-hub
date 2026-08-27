<script lang="ts">
  import { textTrackApi, type Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import { isImage, isPdf, isDocument } from '$lib/utils/mime'
  import TextTrackCreateOCR from './TextTrackCreateOCR.svelte'
  import TextTrackCreateAIDescription from './TextTrackCreateAIDescription.svelte'

  interface Props {
    asset: Asset
    onCreated: () => Promise<void> | void
  }

  let { asset, onCreated }: Props = $props()

  const extractSource = $derived(
    isPdf(asset.mime_type)
      ? 'extract_pdf'
      : isDocument(asset.mime_type)
        ? 'extract_document'
        : asset.mime_type.startsWith('text/')
          ? 'extract_plain'
          : null
  )

  let selectedSource = $state<
    'ocr' | 'ai_image_description' | 'manual' | 'extract'
  >(
    // svelte-ignore state_referenced_locally
    isImage(asset.mime_type) ? 'ocr' : extractSource ? 'extract' : 'manual'
  )
  let creating = $state(false)
  let error = $state('')
  let manualContent = $state('')

  async function createAIDescription(params: {
    model: string
    lang: string
    prompt: string
  }) {
    creating = true
    error = ''
    try {
      await textTrackApi.create(asset.id, {
        source: 'ai_image_description',
        lang: params.lang,
        params,
      })
      await onCreated()
    } catch (e) {
      error = e instanceof Error ? e.message : m.text_tracks_error_generic()
    } finally {
      creating = false
    }
  }

  async function createOCR(params: { lang: string; output_format: string }) {
    creating = true
    error = ''
    try {
      await textTrackApi.create(asset.id, {
        source: 'ocr',
        lang: params.lang,
        params,
      })
      await onCreated()
    } catch (e) {
      error = e instanceof Error ? e.message : m.text_tracks_error_generic()
    } finally {
      creating = false
    }
  }

  async function createManual() {
    creating = true
    error = ''
    try {
      await textTrackApi.create(asset.id, {
        source: 'manual',
        params: { content: manualContent },
      })
      manualContent = ''
      await onCreated()
    } catch (e) {
      error = e instanceof Error ? e.message : m.text_tracks_error_generic()
    } finally {
      creating = false
    }
  }

  async function createExtract() {
    if (!extractSource) return
    creating = true
    error = ''
    try {
      await textTrackApi.create(asset.id, { source: extractSource })
      await onCreated()
    } catch (e) {
      error = e instanceof Error ? e.message : m.text_tracks_error_generic()
    } finally {
      creating = false
    }
  }
</script>

<div
  class="space-y-4 rounded-lg border border-[var(--border)] bg-[var(--bg-elevated)] p-4"
>
  {#if isImage(asset.mime_type) || extractSource}
    <label class="block space-y-2">
      <span class="text-sm font-medium text-[var(--text-primary)]">
        {m.text_tracks_source_label()}
      </span>
      <select
        bind:value={selectedSource}
        class="w-full rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 text-sm text-[var(--text-primary)]"
      >
        {#if isImage(asset.mime_type)}
          <option value="ocr">{m.text_tracks_source_ocr()}</option>
          <option value="ai_image_description"
            >{m.text_tracks_source_ai_image_description()}</option
          >
        {/if}
        {#if extractSource}
          <option value="extract">{m.text_tracks_source_extract()}</option>
        {/if}
        <option value="manual">{m.text_tracks_source_manual()}</option>
      </select>
    </label>
  {/if}

  {#if selectedSource === 'ocr'}
    <TextTrackCreateOCR {creating} onCreate={createOCR} />
  {:else if selectedSource === 'ai_image_description'}
    <TextTrackCreateAIDescription {creating} onCreate={createAIDescription} />
  {:else if selectedSource === 'extract'}
    <button
      type="button"
      class="w-full rounded-lg bg-[var(--accent-cta)] px-4 py-2 text-sm font-medium text-white transition hover:bg-[var(--accent-cta-hover)] disabled:cursor-not-allowed disabled:opacity-50"
      disabled={creating}
      onclick={createExtract}
    >
      {creating ? m.queuing_() : m.text_tracks_extract_submit()}
    </button>
  {:else}
    <label class="block space-y-2">
      <span class="text-sm font-medium text-[var(--text-primary)]">
        {m.text_tracks_manual_content_label()}
      </span>
      <textarea
        bind:value={manualContent}
        rows="5"
        class="w-full rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 dark:focus:ring-indigo-900"
        placeholder={m.text_tracks_manual_content_placeholder()}
      ></textarea>
    </label>
    <button
      type="button"
      class="w-full rounded-lg bg-[var(--accent-cta)] px-4 py-2 text-sm font-medium text-white transition hover:bg-[var(--accent-cta-hover)] disabled:cursor-not-allowed disabled:opacity-50"
      disabled={creating}
      onclick={createManual}
    >
      {creating ? m.queuing_() : m.text_tracks_manual_submit()}
    </button>
  {/if}

  {#if error}
    <p class="text-sm text-[var(--accent-danger)]">{error}</p>
  {/if}
</div>
