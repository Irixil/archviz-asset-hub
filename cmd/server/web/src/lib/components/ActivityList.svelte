<script lang="ts">
  import { Inbox } from '@lucide/svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import Feedback from './ui/Feedback.svelte'
  import ButtonShowMore from './ui/ButtonShowMore.svelte'
  import Actor from './ui/Actor.svelte'
  import type { AuditEvent } from '$lib/api'
  import { m } from '$lib/paraglide/messages'

  let {
    loading,
    error,
    events,
    hasMore,
    loadMore,
    loadingMore,
    class: extraClass,
  }: {
    loading: boolean
    error: string
    events: AuditEvent[]
    hasMore: boolean
    loadMore: () => void
    loadingMore: boolean
    class?: string
  } = $props()

  function relativeTime(isoOrSqlite: string): string {
    const date = new Date(
      isoOrSqlite.replace(' ', 'T') + (isoOrSqlite.includes('T') ? '' : 'Z')
    )
    const diffMs = Date.now() - date.getTime()
    const diffSec = Math.floor(diffMs / 1000)
    if (diffSec < 60) return m.just_now()
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)} 分钟前`
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)} 小时前`
    if (diffSec < 86400 * 7) return `${Math.floor(diffSec / 86400)} 天前`
    return date.toLocaleDateString('zh-CN')
  }

  function eventText(text: string): string {
    const fieldUpdate = text.match(/^set (.+) to (.+)$/)
    if (fieldUpdate) return `将${fieldUpdate[1]}设为${fieldUpdate[2]}`
    return text
  }
</script>

<div class={`flex flex-col ${extraClass}`}>
  {#if loading}
    <div class="flex justify-center py-12">
      <Spinner size="md" />
    </div>
  {:else if error}
    <Feedback {error} />
  {:else if events.length === 0}
    <div
      class="flex flex-col items-center gap-3 py-12 text-center text-zinc-400 dark:text-zinc-500"
    >
      <Inbox class="h-10 w-10" />
      <p class="text-md">{m.no_activity_yet()}</p>
    </div>
  {:else}
    <ul class="divide-y divide-zinc-100 dark:divide-zinc-800">
      {#each events as event (event.id)}
        <li class="flex items-start gap-3 px-5 py-3">
          <Actor actor={event.actor} class="h-6 w-6" />
          <div class="min-w-0 flex-1">
            <p class="text-md text-zinc-800 dark:text-zinc-200">
              {#if event.actor.type === 'user' && event.actor.name}
                <span class="font-medium">{event.actor.name}</span>
                {' '}
              {:else if event.actor.type === 'system'}
                <span
                  class="font-medium text-zinc-500 italic dark:text-zinc-400"
                  >{m.system()}</span
                >
                {' '}
              {/if}
              <span class="text-zinc-600 dark:text-zinc-400"
                >{eventText(event.human_readable)}</span
              >
            </p>
            <p class="mt-0.5 text-sm text-zinc-400 dark:text-zinc-500">
              {relativeTime(event.created_at)}
            </p>
          </div>
        </li>
      {/each}
    </ul>

    {#if hasMore}
      <ButtonShowMore onclick={loadMore} loading={loadingMore} />
    {/if}
  {/if}
</div>
