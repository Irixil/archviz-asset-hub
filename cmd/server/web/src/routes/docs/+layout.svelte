<script lang="ts">
  import DocsSidebar from '$lib/components/docs/DocsSidebar.svelte'
  import DocsSearch from '$lib/components/docs/DocsSearch.svelte'
  import DiamondIcon from '$lib/components/ui/DiamondIcon.svelte'
  import { afterNavigate } from '$app/navigation'
  import { page } from '$app/state'
  import { navForSection, type DocSection } from '$lib/docs/nav'

  let { children } = $props()

  let mainEl: HTMLElement

  afterNavigate(() => {
    mainEl?.scrollTo({ top: 0 })
  })

  const segments = $derived(page.url.pathname.split('/').filter(Boolean))
  const section = $derived((segments[1] as DocSection) || 'help')
  const slug = $derived(segments[2] || 'index')

  const sectionLabel: Record<DocSection, string> = {
    help: 'User guide',
    tech: 'Self-hosting',
  }

  const nav = $derived(navForSection(section))
  const currentEntry = $derived(nav.find((e) => e.slug === slug))
</script>

<div class="docs-shell">
  <aside class="docs-aside">
    <div class="docs-aside-header">
      <a href="/" class="docs-logo">
        <DiamondIcon letter="D" size={32} />
        <div class="docs-logo-text">
          <span class="docs-logo-name">Damask</span>
          <span class="docs-logo-sub">Documentation</span>
        </div>
      </a>
      <DocsSearch />
    </div>
    <DocsSidebar />
  </aside>

  <main class="docs-main" bind:this={mainEl}>
    {#if segments.length >= 2}
      <nav class="docs-breadcrumb" aria-label="Breadcrumb">
        <a href="/docs/help">Docs</a>
        <span class="docs-breadcrumb-sep">›</span>
        <a href="/docs/{section}">{sectionLabel[section]}</a>
        {#if currentEntry && slug !== 'index'}
          <span class="docs-breadcrumb-sep">›</span>
          <span>{currentEntry.title}</span>
        {/if}
      </nav>
    {/if}

    {@render children()}
  </main>
</div>

<style>
  .docs-shell {
    display: flex;
    height: 100vh;
    background: var(--bg-app);
    overflow: hidden;
  }

  .docs-aside {
    width: 260px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    background: var(--bg-sidebar);
    border-right: 1px solid var(--border-subtle);
    overflow: hidden;
  }

  .docs-aside-header {
    padding: 0.85rem 0.85rem 0.85rem;
    border-bottom: 1px solid var(--border-subtle);
    display: flex;
    flex-direction: column;
    gap: 0.625rem;
  }

  .docs-logo {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    padding: 0.25rem 0.25rem;
    text-decoration: none;
    border-radius: 8px;
    transition: background 0.15s ease;
  }

  .docs-logo:hover {
    background: var(--bg-hover);
  }

  .docs-logo-text {
    display: flex;
    flex-direction: column;
    gap: 0;
  }

  .docs-logo-name {
    font-weight: 700;
    font-size: 0.9375rem;
    color: var(--text-primary);
    letter-spacing: -0.01em;
    line-height: 1.2;
  }

  .docs-logo-sub {
    font-size: 0.6875rem;
    color: var(--text-muted);
    letter-spacing: 0.02em;
    line-height: 1.2;
  }

  .docs-main {
    flex: 1;
    min-width: 0;
    overflow-y: auto;
    padding: 2.5rem 3rem 5rem;
    scroll-padding-top: 4rem;
  }

  .docs-breadcrumb {
    display: flex;
    align-items: center;
    gap: 0.3125rem;
    font-size: 0.8125rem;
    color: var(--text-muted);
    margin-bottom: 1.75rem;
  }

  .docs-breadcrumb a {
    color: var(--text-secondary);
    text-decoration: none;
    transition: color 0.12s ease;
  }

  .docs-breadcrumb a:hover {
    color: var(--text-primary);
  }

  .docs-breadcrumb-sep {
    color: var(--border);
  }
</style>
