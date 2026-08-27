<script lang="ts">
  import {
    ChevronDown,
    Plus,
    Settings,
    Share2,
    Users,
    Image,
    Check,
  } from '@lucide/svelte'
  import { workspaceApi, type WorkspaceWithRole } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { m } from '$lib/paraglide/messages'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { onMount } from 'svelte'
  import { fly } from 'svelte/transition'
  import { cubicOut } from 'svelte/easing'
  import DiamondIcon from '$lib/components/ui/DiamondIcon.svelte'

  function wsColor(_name: string): [string, string] {
    return ['#9f6215', '#d8a14b']
  }

  // hex → "r g g" for use in rgba()
  function hexToRGB(hex: string): string {
    const r = parseInt(hex.slice(1, 3), 16)
    const g = parseInt(hex.slice(3, 5), 16)
    const b = parseInt(hex.slice(5, 7), 16)
    return `${r} ${g} ${b}`
  }

  let open = $state(false)
  let workspaces = $state<WorkspaceWithRole[]>([])
  let switching = $state(false)
  let visibleItems = $state<boolean[]>([])

  interface Props {
    class?: string
  }

  let { class: extraClass = '' }: Props = $props()

  const activeWs = $derived.by(() => {
    const id = authStore.workspace?.id
    return workspaces.find((w) => w.id === id)
  })
  const otherWs = $derived.by(() => {
    const id = authStore.workspace?.id
    return workspaces.filter((w) => w.id !== id)
  })

  onMount(loadWorkspaces)

  async function loadWorkspaces() {
    try {
      workspaces = await workspaceApi.list()
    } catch {
      // keep empty — dropdown still shows current workspace
    }
  }

  function toggle() {
    if (!open) {
      loadWorkspaces()
      visibleItems = []
      scheduleStagger()
    }
    open = !open
  }

  function scheduleStagger() {
    const delays = [
      60,
      ...otherWs.map((_, i) => 90 + i * 28),
      90 + otherWs.length * 28,
    ]
    delays.forEach((delay) => {
      setTimeout(() => {
        visibleItems = [...visibleItems, true]
      }, delay)
    })
  }

  function handleOutsideClick(e: MouseEvent) {
    if (!(e.target as Element).closest('[data-workspace-switcher]')) {
      open = false
    }
  }

  async function switchTo(ws: WorkspaceWithRole) {
    if (ws.id === authStore.workspace?.id) {
      open = false
      return
    }
    switching = true
    try {
      const res = await workspaceApi.switch(ws.id)
      authStore.switchWorkspace(res.workspace, res.role)
      undoStore.clear()
      open = false
      window.location.reload()
    } catch {
      switching = false
    }
  }

  async function createNew() {
    const name = prompt(m.workspace_prompt_name())
    if (!name?.trim()) return
    try {
      await workspaceApi.createWorkspace(name.trim())
      loadWorkspaces()
    } catch {
      // silently ignore
    }
  }

  const initials = $derived(
    authStore.workspace?.name?.[0]?.toUpperCase() ?? 'W'
  )
</script>

<svelte:window onclick={handleOutsideClick} />

<div class="relative {extraClass}" data-workspace-switcher>
  <button
    onclick={toggle}
    class="flex w-full items-center gap-2.5 rounded-lg px-1 py-1 transition-colors hover:bg-[var(--bg-hover)]"
    aria-expanded={open}
  >
    <div class="flex w-full items-center gap-2">
      <!-- Trigger diamond: pulse-open + hover glow keyed to workspace color -->
      <div
        class="ws-diamond-trigger ws-gem shrink-0"
        class:ws-diamond-open={open}
        style="--gem-rgb: {hexToRGB(
          wsColor(authStore.workspace?.name ?? 'W')[1]
        )}"
      >
        <DiamondIcon
          letter={initials}
          color={wsColor(authStore.workspace?.name ?? 'W')[0]}
          bright={wsColor(authStore.workspace?.name ?? 'W')[1]}
          size={32}
        />
      </div>
      <div class="flex flex-1 flex-col truncate text-left">
        <span
          class="truncate text-base font-semibold text-[var(--text-primary)]"
        >
          {authStore.workspace?.name ?? 'Workspace'}
        </span>
        <span class="text-xs text-[var(--text-muted)] tabular-nums">
          {#if activeWs}
            {activeWs.member_count} {m.members()}
          {:else}
            &nbsp;
          {/if}
        </span>
      </div>
      <ChevronDown
        class="ws-chevron mr-1 h-4 w-4 shrink-0 text-[var(--text-muted)] {open
          ? 'ws-chevron-open'
          : ''}"
      />
    </div>
  </button>

  {#if open}
    <div
      class="absolute top-full left-1 z-50 w-80 rounded-xl border border-[var(--border)] bg-[var(--bg-surface)] py-2 shadow-lg"
      role="menu"
      transition:fly={{ y: -6, duration: 200, easing: cubicOut }}
    >
      <!-- Active workspace — tall card -->
      {#if activeWs}
        {@const [color, bright] = wsColor(activeWs.name)}
        <div
          class="ws-stagger-item mx-2 mb-2 rounded-lg bg-[var(--accent-soft)] p-3"
          class:ws-stagger-visible={visibleItems[0]}
        >
          <div class="mb-2 flex items-center gap-2">
            <!-- Active gem: slow ambient pulse -->
            <div
              class="ws-gem ws-gem-active shrink-0"
              style="--gem-rgb: {hexToRGB(bright)}"
            >
              <DiamondIcon
                letter={activeWs.name[0]?.toUpperCase() ?? 'W'}
                {color}
                {bright}
                size={28}
              />
            </div>

            <div
              class="my-1 flex-1 truncate text-sm font-semibold text-[var(--text-primary)]"
            >
              {activeWs.name}
            </div>

            <Check class="h-4 w-4 shrink-0 text-[var(--accent)]" />
          </div>

          <div class="my-3 flex gap-3 text-xs text-[var(--accent-text)]">
            <span class="flex items-center gap-1">
              <Users class="h-3.5 w-3.5" />
              {activeWs.member_count}
              {m.members()}
            </span>
            <span class="flex items-center gap-1">
              <Image class="h-3.5 w-3.5" />
              {m.assets_count({ count: activeWs.asset_count })}
            </span>
          </div>

          <div class="mt-2 flex items-center gap-2">
            <a
              href="/library/shares"
              onclick={() => (open = false)}
              class="flex items-center gap-1 rounded-md bg-[var(--bg-elevated)] px-2 py-1 text-xs font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]"
            >
              <Share2 class="h-3 w-3" />
              {m.shares()}
            </a>
            <a
              href="/library/settings/members"
              onclick={() => (open = false)}
              class="flex items-center gap-1 rounded-md bg-[var(--bg-elevated)] px-2 py-1 text-xs font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]"
            >
              <Settings class="h-3 w-3" />
              {m.settings()}
            </a>
          </div>
        </div>
      {/if}

      <!-- Other workspaces — compact rows -->
      {#if otherWs.length > 0}
        <p
          class="px-3 pt-0.5 pb-1 text-xs font-semibold tracking-widest text-[var(--text-muted)]"
        >
          {m.workspaces()}
        </p>
        {#each otherWs as ws, idx (ws.id)}
          {@const [color, bright] = wsColor(ws.name)}
          <button
            onclick={() => switchTo(ws)}
            disabled={switching}
            class="ws-ws-row text-md ws-stagger-item flex w-full items-center gap-3 px-5 py-3 text-left text-[var(--text-secondary)] transition-colors"
            class:ws-stagger-visible={visibleItems[idx + 1]}
            style="--row-rgb: {hexToRGB(bright)}"
            role="menuitem"
          >
            <div class="ws-gem shrink-0" style="--gem-rgb: {hexToRGB(bright)}">
              <DiamondIcon
                letter={ws.name[0]?.toUpperCase() ?? 'W'}
                {color}
                {bright}
                size={28}
              />
            </div>
            <span class="flex-1 truncate font-medium">{ws.name}</span>
            <span
              class="flex items-center gap-1 text-xs text-[var(--text-muted)]"
            >
              <Users class="h-3 w-3" />
              {ws.member_count}
            </span>
          </button>
        {/each}
      {/if}

      <div class="my-1 border-t border-[var(--border-subtle)]"></div>

      <!-- Add workspace row -->
      <button
        onclick={createNew}
        class="ws-add-row text-md ws-stagger-item flex w-full items-center gap-3 px-5 py-2 text-[var(--text-muted)] transition-colors hover:text-[var(--text-primary)]"
        class:ws-stagger-visible={visibleItems[otherWs.length + 1]}
        role="menuitem"
      >
        <div
          class="ws-add-ring flex h-7 w-7 shrink-0 items-center justify-center rounded-lg border border-dashed border-[var(--border)]"
        >
          <Plus class="h-3.5 w-3.5" />
        </div>
        <span>{m.add_new_workspace()}</span>
      </button>
    </div>
  {/if}
</div>

<style>
  /* ── Chevron ─────────────────────────────────────────────── */
  :global(.ws-chevron) {
    transition: transform 220ms cubic-bezier(0.16, 1, 0.3, 1);
  }
  :global(.ws-chevron-open) {
    transform: rotate(180deg);
  }

  /* ── Trigger diamond pulse on open ──────────────────────── */
  .ws-diamond-trigger {
    transition: transform 180ms cubic-bezier(0.16, 1, 0.3, 1);
  }
  .ws-diamond-open {
    animation: ws-diamond-pulse 220ms cubic-bezier(0.16, 1, 0.3, 1) forwards;
  }
  @keyframes ws-diamond-pulse {
    0% {
      transform: scale(1);
    }
    38% {
      transform: scale(0.86);
    }
    100% {
      transform: scale(1);
    }
  }

  /* ── Gem glow — shared by trigger, active card, list rows ── */
  /*
   * --gem-rgb is "r g b" passed inline from the workspace bright color.
   * Glow fires when the parent interactive element (button/row) is hovered,
   * not just the small icon itself.
   */
  .ws-gem {
    transition: filter 240ms cubic-bezier(0.16, 1, 0.3, 1);
  }
  /* Trigger button: hovering anywhere on the button glows the gem */
  button:hover .ws-diamond-trigger,
  /* Dropdown rows: hovering the row glows its gem */
  .ws-ws-row:hover .ws-gem {
    filter: drop-shadow(0 0 6px rgba(var(--gem-rgb) / 0.7))
      drop-shadow(0 0 12px rgba(var(--gem-rgb) / 0.35));
  }

  /* Active workspace gem: slow ambient breathe, always on */
  .ws-gem-active {
    animation: ws-gem-breathe 3s ease-in-out infinite;
  }
  @keyframes ws-gem-breathe {
    0%,
    100% {
      filter: drop-shadow(0 0 3px rgba(var(--gem-rgb) / 0.3));
    }
    50% {
      filter: drop-shadow(0 0 8px rgba(var(--gem-rgb) / 0.65))
        drop-shadow(0 0 16px rgba(var(--gem-rgb) / 0.25));
    }
  }

  /* ── Check mark draw-on ─────────────────────────────────── */
  .ws-check-path {
    stroke-dasharray: 20;
    stroke-dashoffset: 20;
    animation: ws-check-draw 280ms 120ms cubic-bezier(0.16, 1, 0.3, 1) forwards;
  }
  @keyframes ws-check-draw {
    to {
      stroke-dashoffset: 0;
    }
  }

  /* ── Workspace row: tinted hover bg from own accent color ── */
  .ws-ws-row {
    /* fallback for browsers without color-mix */
    background-color: transparent;
    transition: background-color 160ms ease-out;
  }
  .ws-ws-row:hover {
    background-color: rgba(var(--row-rgb) / 0.08);
  }

  /* ── "Add workspace" ring spin on hover ─────────────────── */
  .ws-add-ring {
    transition:
      border-color 200ms ease-out,
      transform 360ms cubic-bezier(0.16, 1, 0.3, 1);
  }
  .ws-add-row:hover .ws-add-ring {
    border-color: oklch(55% 0.18 264);
    transform: rotate(90deg);
  }
  .ws-add-row:hover .ws-add-ring :global(svg) {
    color: oklch(55% 0.18 264);
    transition: color 200ms ease-out;
  }

  /* ── Stagger entrance ───────────────────────────────────── */
  .ws-stagger-item {
    opacity: 0;
    transform: translateY(5px);
    transition:
      opacity 180ms cubic-bezier(0.16, 1, 0.3, 1),
      transform 180ms cubic-bezier(0.16, 1, 0.3, 1);
  }
  .ws-stagger-visible {
    opacity: 1;
    transform: translateY(0);
  }

  /* ── Reduced motion ─────────────────────────────────────── */
  @media (prefers-reduced-motion: reduce) {
    .ws-gem,
    .ws-gem-active,
    .ws-diamond-trigger,
    .ws-diamond-open,
    .ws-stagger-item,
    .ws-add-ring {
      animation: none !important;
      transition: none !important;
    }
    .ws-stagger-item {
      opacity: 1 !important;
      transform: none !important;
    }
    .ws-check-path {
      animation: none !important;
      stroke-dashoffset: 0 !important;
    }
    :global(.ws-chevron) {
      transition: none !important;
    }
  }
</style>
