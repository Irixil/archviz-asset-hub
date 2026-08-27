<script lang="ts">
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { LibraryBig, Menu, Search, Share2 } from '@lucide/svelte'
  import { getContext } from 'svelte'
  import { m } from '$lib/paraglide/messages'
  import {
    APPSHELL_KEY,
    type AppShellContext,
  } from '$lib/components/AppShell.svelte'

  const { openDrawer } = getContext<AppShellContext>(APPSHELL_KEY)

  const tabs = [
    { key: 'library', Icon: LibraryBig, href: '/library' },
    { key: 'shares', Icon: Share2, href: '/library/shares' },
  ] as const

  function isActive(href: string) {
    return href === '/library'
      ? page.url.pathname === href
      : page.url.pathname.startsWith(href)
  }

  async function focusSearch() {
    if (!page.url.pathname.startsWith('/library')) {
      await goto('/library')
      requestAnimationFrame(() => {
        document.querySelector<HTMLInputElement>('[data-search]')?.focus()
      })
      return
    }

    document.querySelector<HTMLInputElement>('[data-search]')?.focus()
  }
</script>

<nav class="mobile-nav" aria-label={m.mobile_nav_label()}>
  {#each tabs as tab}
    {@const active = isActive(tab.href)}
    <a
      href={tab.href}
      class="nav-tab"
      class:active
      aria-current={active ? 'page' : undefined}
    >
      <tab.Icon size={18} />
      <span>{tab.key === 'library' ? m.library() : m.shares()}</span>
    </a>
  {/each}

  <button type="button" class="nav-tab" onclick={focusSearch}>
    <Search size={18} />
    <span>{m.search()}</span>
  </button>

  <button type="button" class="nav-tab" onclick={openDrawer}>
    <Menu size={18} />
    <span>{m.menu()}</span>
  </button>
</nav>

<style>
  .mobile-nav {
    --mobile-nav-height: 64px;
    position: fixed;
    right: 0;
    bottom: 0;
    left: 0;
    z-index: 45;
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    align-items: stretch;
    border-top: 1px solid var(--border-subtle);
    background: color-mix(in oklab, var(--bg-sidebar) 90%, white 10%);
    padding-bottom: env(safe-area-inset-bottom);
    backdrop-filter: blur(18px);
  }

  .nav-tab {
    display: flex;
    min-height: var(--mobile-nav-height);
    min-width: 44px;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.25rem;
    border: 0;
    background: transparent;
    color: var(--text-muted);
    font-size: 10px;
    font-weight: 600;
    text-decoration: none;
  }

  .nav-tab.active {
    color: var(--accent);
  }
</style>
