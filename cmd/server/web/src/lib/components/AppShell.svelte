<script module lang="ts">
  export const APPSHELL_KEY = Symbol('appshell')

  export interface AppShellContext {
    openDrawer: () => void
    closeDrawer: () => void
    isDrawerOpen: () => boolean
  }
</script>

<script lang="ts">
  import { setContext, type Snippet } from 'svelte'
  import MobileDrawer from '$lib/components/MobileDrawer.svelte'
  import MobileNav from '$lib/components/MobileNav.svelte'
  import { viewportStore } from '$lib/stores/viewport.svelte'

  interface Props {
    children: Snippet
    sidebar: Snippet
    showSidebar?: boolean
  }

  let { children, sidebar, showSidebar = true }: Props = $props()

  let drawerOpen = $state(false)

  setContext<AppShellContext>(APPSHELL_KEY, {
    openDrawer: () => {
      drawerOpen = true
    },
    closeDrawer: () => {
      drawerOpen = false
    },
    isDrawerOpen: () => drawerOpen,
  })

  $effect(() => {
    if (!viewportStore.isMobile) drawerOpen = false
  })
</script>

{#if viewportStore.isMobile}
  <div class="mobile-shell">
    <main class="mobile-main">
      {@render children()}
    </main>
    <MobileNav />
    <MobileDrawer
      open={drawerOpen}
      onclose={() => {
        drawerOpen = false
      }}
    >
      {@render sidebar()}
    </MobileDrawer>
  </div>
{:else}
  <div class="desktop-shell">
    {#if showSidebar}
      {@render sidebar()}
    {/if}
    <main class="desktop-main">
      {@render children()}
    </main>
  </div>
{/if}

<style>
  .mobile-shell {
    display: flex;
    flex-direction: column;
    height: 100dvh;
    overflow: hidden;
  }

  .mobile-main {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding-bottom: calc(
      var(--mobile-nav-height, 64px) + env(safe-area-inset-bottom)
    );
  }

  .desktop-shell {
    display: flex;
    height: 100dvh;
    background: var(--bg-app);
  }

  .desktop-main {
    position: relative;
    display: flex;
    min-width: 0;
    flex: 1;
    flex-direction: column;
    overflow: hidden;
  }
</style>
