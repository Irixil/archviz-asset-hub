<script lang="ts">
  import { onMount } from 'svelte'
  import { page } from '$app/state'
  import { navForSection, type DocSection } from '$lib/docs/nav'
  import { BookOpen, Server } from '@lucide/svelte'

  const sections: { id: DocSection; label: string; icon: typeof BookOpen }[] = [
    { id: 'help', label: 'User guide', icon: BookOpen },
    { id: 'tech', label: 'Self-hosting', icon: Server },
  ]

  let collapsed = $state<Record<DocSection, boolean>>({
    help: false,
    tech: false,
  })

  function toggleSection(id: DocSection) {
    collapsed[id] = !collapsed[id]
    try {
      localStorage.setItem('docs-sidebar-collapsed', JSON.stringify(collapsed))
    } catch {}
  }

  onMount(() => {
    try {
      const stored = localStorage.getItem('docs-sidebar-collapsed')
      if (stored) collapsed = JSON.parse(stored)
    } catch {}
  })

  function isActive(section: DocSection, slug: string): boolean {
    const p = page.url.pathname
    if (slug === 'index')
      return p === `/docs/${section}` || p === `/docs/${section}/index`
    return p === `/docs/${section}/${slug}`
  }
</script>

<nav class="docs-sidebar-nav" aria-label="Documentation navigation">
  {#each sections as { id, label, icon: Icon }}
    <div class="docs-sidebar-section">
      <button
        class="docs-sidebar-section-header"
        onclick={() => toggleSection(id)}
        aria-expanded={!collapsed[id]}
      >
        <Icon size={14} />
        <span>{label}</span>
        <svg
          class="docs-sidebar-chevron"
          class:rotated={collapsed[id]}
          width="12"
          height="12"
          viewBox="0 0 12 12"
          fill="none"
        >
          <path
            d="M2 4l4 4 4-4"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
          />
        </svg>
      </button>

      {#if !collapsed[id]}
        <ul class="docs-sidebar-list">
          {#each navForSection(id) as entry}
            <li>
              <a
                href={entry.slug === 'index'
                  ? `/docs/${id}`
                  : `/docs/${id}/${entry.slug}`}
                class="docs-sidebar-link"
                class:active={isActive(id, entry.slug)}
                aria-current={isActive(id, entry.slug) ? 'page' : undefined}
              >
                {entry.title}
              </a>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/each}
</nav>

<style>
  .docs-sidebar-nav {
    flex: 1;
    overflow-y: auto;
    padding: 0.75rem 0.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .docs-sidebar-section {
    display: flex;
    flex-direction: column;
  }

  .docs-sidebar-section + .docs-sidebar-section {
    margin-top: 0.5rem;
  }

  .docs-sidebar-section-header {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    width: 100%;
    padding: 0.3125rem 0.5rem;
    font-size: 0.6875rem;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    background: none;
    border: none;
    cursor: pointer;
    border-radius: 6px;
    text-align: left;
    transition:
      color 0.1s ease,
      background 0.1s ease;
  }

  .docs-sidebar-section-header:hover {
    color: var(--text-secondary);
    background: var(--bg-hover);
  }

  .docs-sidebar-chevron {
    margin-left: auto;
    color: var(--text-muted);
    transition: transform 0.15s ease;
  }

  .docs-sidebar-chevron.rotated {
    transform: rotate(-90deg);
  }

  .docs-sidebar-list {
    list-style: none;
    margin: 0;
    padding: 0 0 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  .docs-sidebar-link {
    display: block;
    padding: 0.3125rem 0.75rem;
    font-size: 0.8125rem;
    color: var(--text-secondary);
    text-decoration: none;
    border-radius: 6px;
    line-height: 1.4;
    transition:
      background 0.1s ease,
      color 0.1s ease;
  }

  .docs-sidebar-link:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .docs-sidebar-link.active {
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 500;
  }
</style>
