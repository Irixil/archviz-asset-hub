<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import { LogOut, Settings } from '@lucide/svelte'
  import { m } from '$lib/paraglide/messages.js'

  // Deterministic hue from string (matches WorkspaceSwitcher palette feel)
  const AVATAR_PALETTES: [string, string][] = [
    ['oklch(48% 0.18 264)', 'oklch(92% 0.06 264)'], // indigo
    ['oklch(46% 0.14 182)', 'oklch(92% 0.05 182)'], // teal
    ['oklch(48% 0.18 300)', 'oklch(92% 0.06 300)'], // purple
    ['oklch(48% 0.18 350)', 'oklch(92% 0.05 350)'], // pink
    ['oklch(52% 0.16 60)', 'oklch(93% 0.05 60)'], // amber
    ['oklch(48% 0.16 210)', 'oklch(92% 0.05 210)'], // cyan
    ['oklch(46% 0.15 145)', 'oklch(92% 0.05 145)'], // green
    ['oklch(46% 0.18 25)', 'oklch(92% 0.05 25)'], // red
  ]

  function avatarColors(name: string): [string, string] {
    let h = 0
    for (let i = 0; i < name.length; i++)
      h = (h * 31 + name.charCodeAt(i)) >>> 0
    return AVATAR_PALETTES[h % AVATAR_PALETTES.length]
  }

  let open = $state(false)
  let menuEl: HTMLDivElement | undefined = $state()
  let imgError = $state(false)

  const user = $derived(authStore.user)
  const initials = $derived(
    (user?.name ?? user?.email ?? '?')
      .split(' ')
      .map((p) => p[0])
      .slice(0, 2)
      .join('')
      .toUpperCase()
  )
  const [fg, bg] = $derived.by(() =>
    avatarColors(user?.name ?? user?.email ?? '')
  )
  const avatarSrc = $derived(
    user?.id ? `/api/v1/users/${user.id}/avatar` : null
  )

  function toggle() {
    open = !open
    imgError = false
  }

  function close() {
    open = false
  }

  function handleOutside(e: MouseEvent) {
    if (menuEl && !menuEl.contains(e.target as Node)) close()
  }

  function handleKey(e: KeyboardEvent) {
    if (e.key === 'Escape') close()
  }

  $effect(() => {
    if (open) {
      document.addEventListener('click', handleOutside, { capture: true })
      document.addEventListener('keydown', handleKey)
      return () => {
        document.removeEventListener('click', handleOutside, { capture: true })
        document.removeEventListener('keydown', handleKey)
      }
    }
  })
</script>

<div class="um-root" bind:this={menuEl}>
  <!-- Trigger -->
  <button
    type="button"
    class="um-trigger"
    onclick={toggle}
    aria-label={m.settings_account_title()}
    aria-expanded={open}
    aria-haspopup="menu"
  >
    <span
      class="um-avatar"
      style="--um-fg: {fg}; --um-bg: {bg}"
      class:um-avatar-open={open}
    >
      {#if avatarSrc && !imgError}
        <img
          src={avatarSrc}
          alt={user?.name ?? ''}
          class="um-avatar-img"
          onerror={() => {
            imgError = true
          }}
        />
      {:else}
        {initials}
      {/if}
    </span>
  </button>

  <!-- Dropdown -->
  {#if open}
    <div class="um-menu" role="menu" tabindex="-1">
      <!-- User identity header -->
      <div class="um-identity">
        <span class="um-identity-avatar" style="--um-fg: {fg}; --um-bg: {bg}">
          {#if avatarSrc && !imgError}
            <img
              src={avatarSrc}
              alt={user?.name ?? ''}
              class="um-avatar-img"
              onerror={() => {
                imgError = true
              }}
            />
          {:else}
            {initials}
          {/if}
        </span>
        <div class="um-identity-info">
          {#if user?.name}
            <p class="um-identity-name">{user.name}</p>
          {/if}
          <p class="um-identity-email">{user?.email ?? ''}</p>
        </div>
      </div>

      <div class="um-divider"></div>

      <a
        href="/library/settings/account"
        role="menuitem"
        class="um-item"
        onclick={close}
      >
        <Settings class="um-item-icon" />
        {m.settings_account_title()}
      </a>

      <div class="um-divider"></div>

      <a
        href="/logout"
        role="menuitem"
        class="um-item um-item-danger"
        onclick={close}
      >
        <LogOut class="um-item-icon" />
        {m.logout()}
      </a>
    </div>
  {/if}
</div>

<style>
  .um-root {
    position: relative;
    flex-shrink: 0;
  }

  /* ── Trigger ── */
  .um-trigger {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 2px;
    border-radius: 50%;
    background: transparent;
    border: none;
    cursor: pointer;
    transition: opacity 0.15s ease;
  }
  .um-trigger:hover {
    opacity: 0.85;
  }
  .um-trigger:focus-visible {
    outline: 2px solid oklch(55% 0.18 264);
    outline-offset: 2px;
  }

  /* ── Avatar circle ── */
  .um-avatar,
  .um-identity-avatar {
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.02em;
    color: var(--um-fg);
    background: var(--um-bg);
    overflow: hidden;
    user-select: none;
    transition:
      box-shadow 0.18s cubic-bezier(0.16, 1, 0.3, 1),
      transform 0.18s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .um-avatar {
    width: 30px;
    height: 30px;
    box-shadow: 0 0 0 1.5px var(--border-subtle, oklch(88% 0.01 264));
  }

  .um-avatar-open,
  .um-trigger:hover .um-avatar {
    box-shadow: 0 0 0 2px oklch(55% 0.18 264 / 0.5);
  }

  .um-identity-avatar {
    width: 36px;
    height: 36px;
    flex-shrink: 0;
  }

  .um-avatar-img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border-radius: 50%;
  }

  /* ── Dropdown ── */
  .um-menu {
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    z-index: 100;
    min-width: 220px;
    border-radius: 10px;
    border: 1px solid var(--border, oklch(88% 0.01 264));
    background: var(--bg-surface, #fff);
    padding: 4px 0;
    box-shadow:
      0 0 0 1px rgb(0 0 0 / 0.04),
      0 4px 8px -2px rgb(0 0 0 / 0.12),
      0 16px 32px -8px rgb(0 0 0 / 0.18);
    animation: um-in 0.14s cubic-bezier(0.25, 1, 0.5, 1) both;
    transform-origin: top right;
  }

  :global(.dark) .um-menu {
    box-shadow:
      0 0 0 1px rgb(0 0 0 / 0.3),
      0 4px 8px -2px rgb(0 0 0 / 0.4),
      0 16px 32px -8px rgb(0 0 0 / 0.5);
  }

  @keyframes um-in {
    from {
      opacity: 0;
      transform: scale(0.95) translateY(-4px);
    }
    to {
      opacity: 1;
      transform: scale(1) translateY(0);
    }
  }

  /* ── Identity header ── */
  .um-identity {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px;
  }

  .um-identity-info {
    min-width: 0;
    flex: 1;
  }

  .um-identity-name {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--text-primary, oklch(20% 0.04 264));
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    margin: 0;
  }

  .um-identity-email {
    font-size: 0.75rem;
    color: var(--text-muted, oklch(60% 0.05 264));
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    margin: 0;
  }

  /* ── Divider ── */
  .um-divider {
    margin: 4px 0;
    height: 1px;
    background: var(--border-subtle, oklch(93% 0.01 264));
  }

  /* ── Menu item ── */
  .um-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 12px;
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--text-primary, oklch(20% 0.04 264));
    text-decoration: none;
    transition: background 0.08s ease;
    cursor: pointer;
  }

  .um-item:hover {
    background: var(--bg-hover, oklch(95% 0.01 264));
  }

  :global(.dark) .um-item:hover {
    background: var(--bg-elevated, oklch(25% 0.03 264));
  }

  .um-item-danger {
    color: oklch(46% 0.18 25);
  }

  :global(.dark) .um-item-danger {
    color: oklch(65% 0.18 25);
  }

  :global(.um-item-icon) {
    width: 14px;
    height: 14px;
    flex-shrink: 0;
    color: var(--text-muted, oklch(60% 0.05 264));
  }

  .um-item-danger :global(.um-item-icon) {
    color: oklch(56% 0.18 25);
  }

  :global(.dark) .um-item-danger :global(.um-item-icon) {
    color: oklch(65% 0.18 25);
  }

  /* ── Reduced motion ── */
  @media (prefers-reduced-motion: reduce) {
    .um-menu {
      animation: none;
    }
    .um-avatar,
    .um-trigger {
      transition: none;
    }
  }
</style>
