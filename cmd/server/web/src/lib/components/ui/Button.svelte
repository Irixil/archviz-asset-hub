<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'outline'
    size?: 'sm' | 'md'
    title?: string
    loading?: boolean
    disabled?: boolean
    type?: 'button' | 'submit' | 'reset'
    onclick?: (e: MouseEvent) => void
    children?: Snippet
    icon?: Snippet
    class?: string
    style?: string
  }

  let {
    title = '',
    variant = 'primary',
    size = 'md',
    loading = false,
    disabled = false,
    type = 'button',
    onclick,
    children,
    icon,
    class: extraClass = '',
    style = '',
  }: Props = $props()

  const base =
    'inline-flex min-h-10 items-center justify-center gap-1.5 rounded-md font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--focus-ring)] focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50'

  const variants: Record<string, string> = {
    primary:
      'bg-[var(--accent-cta)] text-white shadow-sm hover:bg-[var(--accent-cta-hover)] active:bg-[var(--accent-cta-active)]',
    secondary:
      'border border-[var(--border-default)] bg-[var(--bg-surface)] text-[var(--text-primary)] hover:bg-[var(--bg-hover)]',
    ghost: 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]',
    danger:
      'bg-red-600 text-white hover:bg-red-700 focus-visible:ring-red-400 dark:bg-red-500 dark:hover:bg-red-600',
    outline:
      'border border-[var(--border-default)] bg-transparent text-[var(--text-secondary)] hover:border-[var(--accent)] hover:bg-[var(--accent-soft)] hover:text-[var(--accent-text)]',
  }

  const sizes: Record<string, string> = {
    sm: 'px-2.5 py-1.5 text-sm',
    md: 'px-3.5 py-2 text-sm',
  }
</script>

<button
  {type}
  {title}
  disabled={disabled || loading}
  {onclick}
  {style}
  class="{base} {variants[variant]} {sizes[size]} {extraClass}"
>
  {#if loading}
    <svg class="h-3.5 w-3.5 animate-spin" viewBox="0 0 24 24" fill="none">
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
  {:else if icon}
    {@render icon()}
  {/if}
  {@render children?.()}
</button>
