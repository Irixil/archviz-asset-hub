<script lang="ts">
  import { onMount, type Snippet } from 'svelte'
  import { onNavigate } from '$app/navigation'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { sharesStore } from '$lib/stores/shares.svelte'
  import { collectionsStore } from '$lib/stores/collections.svelte'
  import { ingressStore } from '$lib/stores/ingress.svelte'
  import AppShell from '$lib/components/AppShell.svelte'
  import BottomStatusBar from '$lib/components/BottomStatusBar.svelte'
  import LibrarySidebar from '$lib/components/LibrarySidebar.svelte'
  import StackTray from '$lib/components/StackTray.svelte'
  import { useShortcuts } from '$lib/shortcuts'
  import { clearGMode } from '$lib/shortcuts/sequence'
  import { browserDetectStore } from '$lib/stores/browserDetect.svelte'
  import { viewportStore } from '$lib/stores/viewport.svelte'
  import { goto } from '$app/navigation'

  let { children: pageChildren }: { children: Snippet } = $props()

  useShortcuts({
    'search.focus': () =>
      document.querySelector<HTMLInputElement>('[data-search]')?.focus(),
    'upload.open': () =>
      document
        .querySelector<HTMLInputElement>('[data-upload-trigger]')
        ?.click(),
    'sidebar.toggle': () => navigationStore.toggleSidebarVisible(),
    'navigate.library': () => {
      clearGMode()
      goto('/library')
    },
    'navigate.tags': () => {
      clearGMode()
      goto('/library/settings/tags')
    },
    'navigate.settings': () => {
      clearGMode()
      goto('/library/settings/members')
    },
    'navigate.shares': () => {
      clearGMode()
      goto('/library/shares')
    },
  })

  onNavigate((navigation) => {
    if (!document.startViewTransition) return
    const toSettings =
      navigation.to?.url.pathname.startsWith('/library/settings')
    const fromSettings =
      navigation.from?.url.pathname.startsWith('/library/settings')
    if (toSettings === fromSettings) return
    return new Promise((resolve) => {
      document.startViewTransition(async () => {
        resolve()
        await navigation.complete
      })
    })
  })

  let prevNavKey: string | null = null

  $effect(() => {
    const key = [
      navigationStore.activeProjectId,
      navigationStore.activeFolderId,
      navigationStore.activeCollectionId,
      navigationStore.activeArchiveGroupId,
    ].join('|')
    if (key === prevNavKey) return
    prevNavKey = key
    assetsStore.load(true)
  })

  $effect(() => {
    if (projectsStore.stale) projectsStore.load()
  })

  $effect(() => {
    const activeId = navigationStore.activeProjectId
    if (activeId && foldersStore.staleProjects.has(activeId)) {
      foldersStore.loadForProject(activeId)
    }
  })

  onMount(() => {
    browserDetectStore.detectSmallDevice()

    projectsStore.load()
    collectionsStore.load()

    setTimeout(() => {
      sharesStore.load()
      ingressStore.loadSources()
    }, 1000)
  })
</script>

<svelte:head>
  <title>建筑公司内部素材库与工作中台</title>
</svelte:head>

<AppShell showSidebar={navigationStore.sidebarVisible}>
  {#snippet sidebar()}
    <LibrarySidebar />
  {/snippet}

  {#snippet children()}
    <div
      class="relative flex flex-1 flex-col overflow-hidden"
      style="view-transition-name: main-content;"
    >
      {@render pageChildren?.()}

      {#if !viewportStore.isMobile}
        <BottomStatusBar />
      {/if}
      <StackTray />
    </div>
  {/snippet}
</AppShell>

<style>
  @keyframes slide-up-in {
    from {
      transform: translateX(50%);
      opacity: 0;
    }
    to {
      transform: translateX(0);
      opacity: 1;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    :global(::view-transition-group(main-content)),
    :global(::view-transition-old(main-content)),
    :global(::view-transition-new(main-content)) {
      animation: none !important;
    }
  }

  :global(::view-transition-group(main-content)) {
    overflow: hidden;
  }
  :global(::view-transition-image-pair(main-content)) {
    isolation: isolate;
  }
  :global(::view-transition-old(main-content)) {
    z-index: 0;
    animation: none;
    opacity: 0;
  }
  :global(::view-transition-new(main-content)) {
    z-index: 1;
    animation: slide-up-in 300ms cubic-bezier(0.22, 1, 0.36, 1) both;
  }
</style>
