<script lang="ts">
  import { Plus, Image, Share2, Tag, Folder, Sparkles } from '@lucide/svelte'

  const icons: Record<string, typeof Sparkles> = {
    plus: Plus,
    image: Image,
    share: Share2,
    tag: Tag,
    folder: Folder,
    sparkles: Sparkles,
  }

  interface Props {
    icon: string
    title: string
    description: string
    badge?: string | null
    loading?: boolean
    disabled?: boolean
    onSelect: () => void
  }

  let {
    icon,
    title,
    description,
    badge = null,
    loading = false,
    disabled = false,
    onSelect,
  }: Props = $props()

  const IconComponent = $derived(icons[icon] ?? Sparkles)

  function handleClick() {
    if (disabled || loading) return
    onSelect()
  }
</script>

<button
  type="button"
  class="group relative flex h-full flex-col items-start gap-3 overflow-hidden rounded-xl border border-zinc-200 bg-white p-5 text-left transition-colors hover:border-zinc-300 disabled:cursor-not-allowed disabled:opacity-50 dark:border-zinc-800 dark:bg-gray-900 dark:hover:border-zinc-700"
  disabled={disabled || loading}
  onclick={handleClick}
>
  <div
    class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-violet-50 text-violet-500 dark:bg-violet-900/30 dark:text-violet-400"
  >
    <IconComponent class="h-5 w-5" />
  </div>

  <div class="min-w-0">
    <div class="flex flex-wrap items-center gap-2">
      <p class="text-sm font-medium text-[var(--text-primary)]">{title}</p>
      {#if badge}
        <span
          class="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-medium text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400"
        >
          {badge}
        </span>
      {/if}
    </div>
    <p class="mt-1 text-xs leading-relaxed text-[var(--text-secondary)]">
      {description}
    </p>
  </div>

  {#if loading}
    <div
      class="absolute inset-0 flex items-center justify-center bg-white/70 dark:bg-gray-900/70"
    >
      <svg
        class="h-5 w-5 animate-spin text-violet-500"
        viewBox="0 0 24 24"
        fill="none"
      >
        <circle
          class="opacity-25"
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          stroke-width="4"
        />
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
        />
      </svg>
    </div>
  {/if}
</button>
