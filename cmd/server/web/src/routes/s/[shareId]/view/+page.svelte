<script lang="ts">
  import { onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { ChevronLeft, Download, MessageSquare, Send } from '@lucide/svelte'
  import { formatBytes, mimeCategory } from '$lib/api'
  import Button from '$lib/components/ui/Button.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import SharedAsset from '$lib/components/SharedAsset.svelte'
  import {
    publicViewStore as store,
    TYPES_BACKGROUNDS,
  } from '$lib/stores/publicView.svelte'
  import PublicAssetCard from '$lib/components/PublicAssetCard.svelte'
  import PublicFooter from '$lib/components/PublicFooter.svelte'
  import AssetIcon from '$lib/components/asset/AssetIcon.svelte'
  import SharedVariantFilmstrip from '$lib/components/share/SharedVariantFilmstrip.svelte'
  import SharedVariantList from '$lib/components/share/SharedVariantList.svelte'
  import Close from '$lib/components/ui/Close.svelte'
  import ThemeToggle from '$lib/components/ThemeToggle.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import { m } from '$lib/paraglide/messages'
  import { viewportStore } from '$lib/stores/viewport.svelte'
  import { fly, fade } from 'svelte/transition'
  import { cubicOut } from 'svelte/easing'

  let shareId = $derived(page.params.shareId || '')
  let selectedVariantIdx = $state<number | null>(null)
  let selectedAssetId = $state<string | null>(null)
  let commentFormOpen = $state(false)

  const avatarColors = [
    'bg-violet-200 text-violet-700 dark:bg-violet-800 dark:text-violet-200',
    'bg-blue-200 text-blue-700 dark:bg-blue-800 dark:text-blue-200',
    'bg-emerald-200 text-emerald-700 dark:bg-emerald-800 dark:text-emerald-200',
    'bg-amber-200 text-amber-700 dark:bg-amber-800 dark:text-amber-200',
  ]

  function formatDateTime(iso: string) {
    return new Date(iso).toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    })
  }

  function initials(name: string) {
    return name
      .split(' ')
      .slice(0, 2)
      .map((w) => w[0]?.toUpperCase() ?? '')
      .join('')
  }

  function avatarColor(name: string) {
    let hash = 0
    for (let i = 0; i < name.length; i++)
      hash = (hash * 31 + name.charCodeAt(i)) & 0xffffff
    return avatarColors[hash % avatarColors.length]
  }

  onMount(async () => {
    await store.init(shareId)
    store.loadGallery(shareId, () =>
      goto(`/s/${shareId}`, { replaceState: true })
    )
  })

  $effect(() => {
    const assetId = store.selectedAsset?.id ?? null
    if (assetId !== selectedAssetId) {
      selectedAssetId = assetId
      selectedVariantIdx = null
      commentFormOpen = false
    }
  })

  function handleWindowKeydown(e: KeyboardEvent) {
    if (store.panelOpen && (e.key === 'ArrowLeft' || e.key === 'ArrowRight')) {
      e.preventDefault()
      store.navigateAsset(shareId, e.key === 'ArrowLeft' ? 'prev' : 'next')
    }
  }
</script>

<svelte:window onkeydown={handleWindowKeydown} />

<svelte:head>
  <title>{store.share?.label || m.shared_gallery()} — Damask</title>
</svelte:head>

<div class="relative flex min-h-screen flex-col">
  <!-- Header -->
  <header
    class="border-b border-[var(--border)] bg-[var(--bg-surface)] px-4 py-3 sm:px-6 sm:py-4"
  >
    <div
      class="mx-auto flex max-w-6xl flex-col gap-2 sm:flex-row sm:items-center sm:justify-between sm:gap-0"
    >
      <div>
        {#if store.share}
          <h1 class="text-lg font-semibold text-[var(--text-primary)]">
            {store.share.label}
          </h1>
          <div
            class="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0 text-xs font-medium text-[var(--text-muted)]"
          >
            <span
              >{store.assets.length} asset{store.assets.length === 1
                ? ''
                : 's'}</span
            >
            {#if store.visitorName}
              <span class="text-[var(--border)]">•</span>
              <span>Viewing as {store.visitorName}</span>
            {/if}
            {#if store.expiryWarning}
              <span class="text-[var(--border)]">•</span>
              <span
                class="flex items-center gap-1 text-amber-500 dark:text-amber-400"
              >
                <svg
                  class="h-3.5 w-3.5"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <circle cx="12" cy="12" r="10" /><path
                    d="M12 6v6l4 2"
                    stroke-linecap="round"
                  />
                </svg>
                {store.expiryWarning}
              </span>
            {/if}
          </div>
        {:else if store.loadingGallery}
          <div
            class="h-5 w-48 animate-pulse rounded bg-[var(--bg-elevated)]"
          ></div>
          <div
            class="mt-1 h-4 w-32 animate-pulse rounded bg-[var(--bg-elevated)]"
          ></div>
        {/if}
      </div>

      <div class="flex items-center justify-end gap-2 sm:justify-start">
        {#if store.share?.allow_download && store.assets.length > 0}
          <Button
            variant="secondary"
            onclick={() => store.downloadAll(shareId)}
            size="md"
            title={m.download_all()}
          >
            {#snippet icon()}
              <Download class="h-4 w-4" />
            {/snippet}
            <span class="hidden sm:inline">{m.download_all()}</span>
          </Button>
        {/if}

        <ThemeToggle />
      </div>
    </div>
  </header>

  <!-- Body -->
  <main class="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
    {#if store.loadingGallery}
      <div class="flex items-center justify-center py-24">
        <Spinner size="lg" />
      </div>
    {:else if store.galleryError}
      <div class="flex flex-col items-center justify-center py-24 text-center">
        <p class="text-gray-500 dark:text-gray-400">{store.galleryError}</p>
      </div>
    {:else if store.assets.length === 0}
      <div class="flex flex-col items-center justify-center py-24 text-center">
        <p class="text-gray-500 dark:text-gray-400">{m.no_asset_here()}</p>
      </div>
    {:else}
      <div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
        {#each store.assets as asset}
          <PublicAssetCard
            {asset}
            thumbUrl={store.thumbUrl(shareId, asset.id)}
            openAsset={() => store.openAsset(shareId, asset)}
          />
        {/each}
      </div>
    {/if}
  </main>

  <PublicFooter />
</div>

<!-- Review Panel (S12) -->
{#if store.panelOpen && store.selectedAsset}
  {@const category = mimeCategory(store.previewMimeType)}
  <!-- Desktop preview area -->
  <div
    transition:fade={{ duration: 180 }}
    class="preview-stage fixed inset-y-0 left-0 z-40 hidden flex-col bg-black/40 backdrop-blur-sm md:flex md:w-[75%]"
    role="button"
    tabindex="-1"
    onclick={store.closePanel}
    onkeydown={(e) => e.key === 'Escape' && store.closePanel()}
  >
    <div
      class="preview-wrap flex min-h-0 flex-1 items-center justify-center p-16"
    >
      <SharedAsset
        {category}
        asset={{ ...store.selectedAsset, mime_type: store.previewMimeType }}
        thumbUrl={store.previewThumbUrl ??
          store.thumbUrl(shareId, store.selectedAsset.id)}
        assetUrl={store.previewFileUrl ??
          store.fileUrlWithToken(shareId, store.selectedAsset.id)}
      />
    </div>

    {#if store.selectedAsset.shared_variants?.length}
      {@const sharedVariants = [...(store.selectedAsset.shared_variants ?? [])]}
      <div
        class="desktop-filmstrip px-8 py-5 pt-0"
        role="presentation"
        onclick={(event) => event.stopPropagation()}
        onkeydown={(event) => event.stopPropagation()}
      >
        <SharedVariantFilmstrip
          {shareId}
          assetId={store.selectedAsset.id}
          variants={sharedVariants}
          bind:selectedIndex={selectedVariantIdx}
          allowDownload={store.share?.allow_download ?? false}
          onSelect={(index) =>
            store.selectVariant(
              index !== null ? sharedVariants[index] : store.selectedVariant!
            )}
          getThumbUrl={store.variantThumbUrl}
          getDownloadUrl={store.variantFileUrl}
          authHeaders={() =>
            store.sessionToken
              ? { 'X-Share-Token': store.sessionToken }
              : ({} as Record<string, string>)}
        />
      </div>
    {/if}
  </div>

  <!-- Panel -->
  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <aside
    transition:fly={{ x: '100%', duration: 220, easing: cubicOut }}
    class="panel-aside fixed inset-y-0 right-0 z-50 flex w-full max-w-full flex-col md:w-[25%] md:max-w-2xl"
    role="dialog"
    aria-label="Asset review"
  >
    <!-- Panel header -->
    <div class="panel-header flex flex-shrink-0 items-center gap-3 px-4 py-3">
      <button
        class="panel-back-btn flex h-8 w-8 items-center justify-center rounded-lg transition-colors md:hidden"
        onclick={store.closePanel}
        aria-label="Back to gallery"
      >
        <ChevronLeft class="h-5 w-5" />
      </button>
      <AssetIcon {category} />
      <div class="min-w-0 flex-1">
        <p
          class="panel-filename truncate font-semibold"
          title={store.selectedAsset.original_filename}
        >
          {store.selectedAsset.original_filename}
        </p>
        <p class="panel-meta text-sm">
          {formatBytes(store.selectedAsset.size)} · {category.toUpperCase()}
        </p>
      </div>
      <div class="flex items-center gap-1">
        {#if store.share?.allow_download}
          <a
            href={store.fileUrlWithToken(shareId, store.selectedAsset.id)}
            download={store.selectedAsset.original_filename}
            class="panel-icon-btn flex h-8 w-8 items-center justify-center rounded-lg transition-colors"
            aria-label="Download"
          >
            <Download class="h-4 w-4" />
          </a>
        {/if}

        <span class="hidden md:flex">
          <Close close={store.closePanel} />
        </span>
      </div>
    </div>

    <!-- Mobile Preview area -->
    <div
      class="block flex-shrink-0 overflow-hidden md:hidden {TYPES_BACKGROUNDS[
        category
      ]}"
      style="height: 220px"
    >
      <div class="flex h-full items-center justify-center">
        <SharedAsset
          {category}
          asset={{ ...store.selectedAsset, mime_type: store.previewMimeType }}
          thumbUrl={store.previewThumbUrl ??
            store.thumbUrl(shareId, store.selectedAsset.id)}
          assetUrl={store.previewFileUrl ??
            store.fileUrlWithToken(shareId, store.selectedAsset.id)}
        />
      </div>
    </div>

    {#if viewportStore.isMobile && store.selectedAsset.shared_variants?.length}
      {@const sharedVariants = [...(store.selectedAsset.shared_variants ?? [])]}
      <div class="px-4 py-4">
        <SharedVariantList
          {shareId}
          assetId={store.selectedAsset.id}
          variants={sharedVariants}
          selectedVariantId={store.selectedVariant?.id}
          allowDownload={store.share?.allow_download ?? false}
          onselect={(variant) => {
            const index = sharedVariants.findIndex((v) => v.id === variant.id)
            if (index >= 0) selectedVariantIdx = index
            store.selectVariant(variant)
          }}
          getThumbUrl={store.variantThumbUrl}
          getDownloadUrl={store.variantFileUrl}
          authHeaders={() =>
            store.sessionToken
              ? { 'X-Share-Token': store.sessionToken }
              : ({} as Record<string, string>)}
        />
      </div>
    {/if}

    <!-- Comments section -->
    {#if store.share?.allow_comments}
      <div class="flex flex-1 flex-col overflow-hidden">
        <!-- Comments header -->
        <div class="comments-header flex items-center gap-2 px-4 py-3">
          <MessageSquare class="comments-icon h-4 w-4" />
          <span class="comments-title font-semibold">Comments</span>
          {#if store.comments.length > 0}
            <span class="comment-count">
              {store.comments.length}
            </span>
          {/if}
        </div>

        <!-- Comment list -->
        <div class="flex-1 overflow-y-auto px-4">
          {#if store.loadingComments}
            <div class="flex justify-center py-6">
              <Spinner size="sm" />
            </div>
          {:else if store.comments.length === 0}
            <p class="comment-empty py-4 text-center text-sm">
              {m.comment_first()}
            </p>
          {:else}
            <div class="flex flex-col gap-4 pb-4">
              {#each store.comments as comment}
                {@const variantPrefix =
                  comment.body.match(/^@([0-9a-f-]{36}) /)}
                {@const prefixVariant = variantPrefix
                  ? store.selectedAsset?.shared_variants?.find(
                      (v) => v.id === variantPrefix[1]
                    )
                  : null}
                {@const prefixVariantIdx = prefixVariant
                  ? (store.selectedAsset?.shared_variants?.findIndex(
                      (v) => v.id === prefixVariant.id
                    ) ?? -1)
                  : -1}
                <div class="flex gap-3">
                  <div
                    class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full text-sm font-semibold {avatarColor(
                      comment.author_name
                    )}"
                  >
                    {initials(comment.author_name)}
                  </div>
                  <div class="flex-1">
                    <div class="flex items-baseline gap-2">
                      <span class="comment-author text-sm font-semibold"
                        >{comment.author_name}</span
                      >
                      <span class="comment-time text-xs"
                        >{formatDateTime(comment.created_at)}</span
                      >
                    </div>
                    <div
                      class="comment-bubble mt-1 rounded-lg px-3 py-2 text-sm"
                    >
                      {#if prefixVariant}
                        <button
                          class="variant-badge variant-badge--btn"
                          type="button"
                          onclick={() => {
                            if (prefixVariantIdx >= 0)
                              selectedVariantIdx = prefixVariantIdx
                            store.selectVariant(prefixVariant)
                          }}>@{prefixVariant.title}</button
                        >{' '}{comment.body.slice(variantPrefix![0].length)}
                      {:else}
                        {comment.body}
                      {/if}
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Comment form -->
        <div class="comment-form-wrap px-4 py-4">
          <!-- Mobile: collapsible toggle -->
          <button
            class="comment-form-toggle mb-3 flex w-full items-center justify-between text-sm font-medium md:hidden"
            type="button"
            onclick={() => (commentFormOpen = !commentFormOpen)}
            aria-expanded={commentFormOpen}
          >
            <span class="comment-form-label">{m.add_comment()}</span>
            <svg
              class="comment-form-chevron h-4 w-4 transition-transform {commentFormOpen
                ? 'rotate-180'
                : ''}"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              ><path
                d="M6 9l6 6 6-6"
                stroke-linecap="round"
                stroke-linejoin="round"
              /></svg
            >
          </button>
          <!-- Desktop: always-visible label -->
          <p
            class="comment-form-label mb-3 hidden text-sm font-medium md:block"
          >
            {m.add_comment()}
          </p>

          {#if store.commentPosted}
            <div class="comment-posted rounded-lg px-3 py-2 text-sm">
              {m.comment_posted()}
            </div>
          {:else if commentFormOpen || !viewportStore.isMobile}
            <form
              onsubmit={(e) => {
                e.preventDefault()
                store.postComment(shareId)
              }}
            >
              <div class="mb-2">
                <Input
                  placeholder={m.for_notifs()}
                  label={m.email_optional()}
                  type="email"
                  bind:value={store.commentEmail}
                  autocomplete="email"
                />
              </div>
              <div class="mb-3">
                <label
                  for="comment-body"
                  class="comment-form-label mb-1 block text-sm font-medium"
                >
                  {m.message()} *
                </label>
                <textarea
                  bind:value={store.commentBody}
                  placeholder={m.add_feedback()}
                  id="comment-body"
                  rows="3"
                  class="comment-textarea w-full resize-none rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors focus:outline-none {store.commentBodyError
                    ? 'error'
                    : ''}"
                ></textarea>
                <Feedback error={store.commentBodyError} />
              </div>
              <div class="flex justify-end">
                <Button
                  type="submit"
                  variant="primary"
                  loading={store.postingComment}
                  size="md"
                >
                  {#snippet icon()}
                    {#if !store.postingComment}
                      <Send class="h-3.5 w-3.5" />
                    {/if}
                  {/snippet}
                  {m.post_comment()}
                </Button>
              </div>
            </form>
          {/if}
        </div>
      </div>
    {:else}
      <div class="flex-1"></div>
    {/if}
  </aside>
{/if}

<style>
  .panel-aside {
    background: var(--bg-surface);
    box-shadow: 0 20px 60px rgb(0 0 0 / 0.18);
  }
  .preview-stage {
    background: color-mix(in srgb, var(--bg-app) 88%, black);
  }
  .desktop-filmstrip {
    border-top: 1px solid var(--border);
    background: color-mix(in srgb, var(--bg-surface) 82%, transparent);
  }
  .panel-header {
    border-bottom: 1px solid var(--border);
  }
  .panel-filename {
    font-size: 0.875rem;
    color: var(--text-primary);
  }
  .panel-meta {
    color: var(--text-muted);
    margin-top: 1px;
  }
  .panel-back-btn {
    color: var(--text-muted);
  }
  .panel-back-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
  .panel-icon-btn {
    color: var(--text-muted);
  }
  .panel-icon-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
  .comments-header {
    border-bottom: 1px solid var(--border-subtle);
  }
  .comments-icon {
    color: var(--text-muted);
  }
  .comments-title {
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.07em;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .comment-count {
    display: inline-flex;
    align-items: center;
    padding: 1px 7px;
    border-radius: 999px;
    font-size: 0.6875rem;
    font-weight: 600;
    background: var(--bg-elevated);
    color: var(--text-secondary);
  }
  .comment-empty {
    color: var(--text-muted);
  }
  .comment-author {
    color: var(--text-primary);
  }
  .comment-time {
    color: var(--text-muted);
  }
  .comment-bubble {
    background: var(--bg-elevated);
    color: var(--text-primary);
    font-size: 0.875rem;
    line-height: 1.5;
  }
  .comment-form-wrap {
    border-top: 1px solid var(--border);
  }
  .comment-form-label {
    color: var(--text-secondary);
  }
  .comment-form-toggle {
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    color: var(--text-secondary);
  }
  .comment-form-toggle:hover {
    color: var(--text-primary);
  }
  .comment-form-chevron {
    color: var(--text-muted);
  }
  .comment-posted {
    background: color-mix(in srgb, #10b981 10%, transparent);
    color: #059669;
  }
  :global(.dark) .comment-posted {
    background: color-mix(in srgb, #10b981 15%, transparent);
    color: #34d399;
  }
  .comment-textarea {
    background: var(--bg-app);
    border-color: var(--border);
    color: var(--text-primary);
  }
  .comment-textarea::placeholder {
    color: var(--text-muted);
  }
  .comment-textarea:focus {
    border-color: var(--accent-cta);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent-cta) 18%, transparent);
  }
  .comment-textarea.error {
    border-color: var(--accent-danger);
  }
  .comment-textarea.error:focus {
    box-shadow: 0 0 0 2px
      color-mix(in srgb, var(--accent-danger) 18%, transparent);
  }
  .variant-badge {
    display: inline-flex;
    align-items: center;
    padding: 1px 6px;
    border-radius: 999px;
    font-size: 0.6875rem;
    font-weight: 600;
    background: var(--bg-elevated);
    color: var(--text-secondary);
  }
  .variant-badge--btn {
    cursor: pointer;
    border: none;
    font: inherit;
    vertical-align: baseline;
    transition:
      background 0.1s ease,
      color 0.1s ease;
  }
  .variant-badge--btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
</style>
