<script lang="ts">
  import { assetCommentsStore } from '$lib/stores/assetComments.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import type { Asset, Variant } from '$lib/api'
  import { onMount } from 'svelte'
  import { m } from '$lib/paraglide/messages'

  let {
    asset,
    variants,
    onVariantSelect,
  }: {
    asset: Asset
    variants: readonly Variant[]
    onVariantSelect?: (variant: Variant) => void
  } = $props()

  const varRe = /^@([0-9a-f-]{36}) /

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

  const avatarColors = [
    'bg-violet-200 text-violet-700 dark:bg-violet-800 dark:text-violet-200',
    'bg-blue-200 text-blue-700 dark:bg-blue-800 dark:text-blue-200',
    'bg-emerald-200 text-emerald-700 dark:bg-emerald-800 dark:text-emerald-200',
    'bg-amber-200 text-amber-700 dark:bg-amber-800 dark:text-amber-200',
  ]

  function avatarColor(name: string) {
    let hash = 0
    for (let i = 0; i < name.length; i++)
      hash = (hash * 31 + name.charCodeAt(i)) & 0xffffff
    return avatarColors[hash % avatarColors.length]
  }

  onMount(() => {
    assetCommentsStore.load(asset.id)
  })
</script>

<div class="flex-1 overflow-y-auto px-4">
  {#if assetCommentsStore.loading}
    <div class="flex justify-center py-6">
      <Spinner size="sm" />
    </div>
  {:else if assetCommentsStore.comments.length === 0}
    <p class="text-md py-4 text-center text-gray-400 dark:text-gray-600">
      {m.no_comments()}
    </p>
  {:else}
    <div class="flex flex-col gap-4 py-4">
      {#each assetCommentsStore.comments as comment}
        {@const variantPrefix = comment.body.match(varRe)}
        {@const variant = variantPrefix
          ? variants?.find((v) => v.id === variantPrefix[1])
          : null}
        <div class="flex gap-3">
          <div
            class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full text-sm font-bold {avatarColor(
              comment.author_name
            )}"
          >
            {initials(comment.author_name)}
          </div>
          <div class="flex-1">
            <div class="flex items-center gap-2">
              <span class="text-md font-medium text-gray-900 dark:text-gray-100"
                >{comment.author_name}</span
              >
              <span class="text-sm text-gray-400"
                >{formatDateTime(comment.created_at)}</span
              >
            </div>
            <!-- svelte-ignore a11y_click_events_have_key_events -->
            <!-- svelte-ignore a11y_no_static_element_interactions -->
            <div
              class="text-md mt-1 rounded-xl rounded-tl-sm bg-gray-50 px-3 py-2 text-gray-700 dark:bg-gray-800 dark:text-gray-300"
              onclick={() => (variant ? onVariantSelect?.(variant) : undefined)}
            >
              {#if variant}
                {comment.body.replaceAll(variant?.id, (_) => variant.title)}
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

<style>
  .variant-mention {
    display: inline-flex;
    align-items: center;
    padding: 1px 6px;
    border-radius: 999px;
    border: none;
    font-size: 0.6875rem;
    font-weight: 600;
    background: #e5e7eb;
    color: #374151;
    cursor: pointer;
    vertical-align: baseline;
    transition:
      background 0.1s ease,
      color 0.1s ease;
  }
  .variant-mention:hover {
    background: #d1d5db;
    color: #111827;
  }
  :global(.dark) .variant-mention {
    background: #374151;
    color: #d1d5db;
  }
  :global(.dark) .variant-mention:hover {
    background: #4b5563;
    color: #f9fafb;
  }
</style>
