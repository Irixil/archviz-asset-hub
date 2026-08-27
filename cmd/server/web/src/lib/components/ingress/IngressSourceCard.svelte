<script lang="ts">
  import { Clock } from '@lucide/svelte'
  import type { IngressSource, IngressSourceType } from '$lib/api'
  import { ingressStore } from '$lib/stores/ingress.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import SourceTypeIcon from './SourceTypeIcon.svelte'
  import Feedback from '../ui/Feedback.svelte'
  import StatusBadge from '../ui/StatusBadge.svelte'
  import Hint from '../ui/Hint.svelte'
  import SectionHeading from '../ui/SectionHeading.svelte'
  import ButtonDelete from '../ui/ButtonDelete.svelte'
  import ButtonEdit from '../ui/ButtonEdit.svelte'
  import ButtonToggle from '../ui/ButtonToggle.svelte'
  import ButtonStart from '../ui/ButtonStart.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    source: IngressSource
    onedit: (source: IngressSource) => void
    ondelete: (source: IngressSource) => void
  }

  let { source, onedit, ondelete }: Props = $props()

  const SOURCE_LABELS: Record<string, string> = {
    email_api: m.ingress_own_mail(),
    imap: m.ingress_imap(),
    sftp: m.ingress_sftp(),
    dav: m.ingress_dav(),
    s3: m.ingress_s3(),
  }

  const destProject = $derived(
    source.dest_project_id
      ? projectsStore.projects.find((p) => p.id === source.dest_project_id)
      : null
  )

  function formatRelative(iso: string | null): string {
    if (!iso) return m.never()
    const diff = Date.now() - new Date(iso).getTime()
    const mins = Math.floor(diff / 60_000)
    if (mins < 1) return m.just_now()
    if (mins < 60) return `${mins} 分钟前`
    const hrs = Math.floor(mins / 60)
    if (hrs < 24) return `${hrs} 小时前`
    return `${Math.floor(hrs / 24)} 天前`
  }

  let polling = $state(false)
  async function handlePoll() {
    polling = true
    await ingressStore.pollSource(source.id)
    polling = false
  }
</script>

<div
  class="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-800 dark:bg-gray-900"
>
  <div class="flex items-start gap-4 px-5 py-4">
    <!-- Icon -->
    <div
      class="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-indigo-50 text-indigo-500 dark:bg-indigo-900/30 dark:text-indigo-400"
    >
      <SourceTypeIcon type={source.type as IngressSourceType} class="h-5 w-5" />
    </div>

    <!-- Meta -->
    <div class="min-w-0 flex-1">
      <div class="flex flex-wrap items-center gap-2">
        <SectionHeading title={source.label} />
        <Hint>{SOURCE_LABELS[source.type] ?? source.type}</Hint>
        <StatusBadge
          status={!source.enabled
            ? 'disabled'
            : source.last_error
              ? 'error'
              : 'healthy'}
        />
      </div>

      <!-- Destination + last polled -->
      <div
        class="mt-1 flex flex-wrap items-center gap-3 text-sm text-gray-400 dark:text-gray-500"
      >
        {#if destProject}
          <span>→ {destProject.name}</span>
        {/if}
        <span class="inline-flex items-center gap-1">
          <Clock class="h-3 w-3" />
          {formatRelative(source.last_polled_at ?? null)}
        </span>
        <span>每 {source.poll_interval_min} 分钟扫描</span>
      </div>

      <Feedback error={source.last_error} />
    </div>

    <!-- Actions -->
    <div
      class="flex shrink-0 items-center gap-1"
      onclick={(e) => e.stopPropagation()}
      role="none"
    >
      <ButtonStart
        onclick={handlePoll}
        disabled={polling || !source.enabled}
        title={m.poll_now()}
      />
      <ButtonToggle
        enabled={source.enabled}
        onclick={() => ingressStore.toggleSource(source.id, !source.enabled)}
        title={source.enabled ? m.disable() : m.enable()}
      />
      <ButtonEdit onclick={() => onedit(source)} />
      <ButtonDelete onclick={() => ondelete(source)} />
    </div>
  </div>
</div>
