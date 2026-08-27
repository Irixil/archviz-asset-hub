<script lang="ts">
  import { goto } from '$app/navigation'
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { collectionsStore } from '$lib/stores/collections.svelte'
  import {
    ACTION_SHEET_KEY,
    type ActionSheetContext,
  } from '$lib/components/ActionSheet.svelte'
  import SearchInput from '$lib/components/ui/SearchInput.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { taxonomyGroupById } from '$lib/data/architectureTaxonomy'
  import { viewportStore } from '$lib/stores/viewport.svelte'
  import {
    Share2,
    ChevronDown,
    Users,
    Plus,
    Tags,
    Upload,
    Download,
    Settings2,
  } from '@lucide/svelte'
  import { m } from '$lib/paraglide/messages.js'
  import UndoRedo from './UndoRedo.svelte'
  import UserMenu from './ui/UserMenu.svelte'
  import { getContext, type Snippet } from 'svelte'

  type Props = {
    prefix?: Snippet
    onShareProject?: () => void
    showShareButton?: boolean
  }

  let { onShareProject, showShareButton = false, prefix }: Props = $props()

  let addMenuOpen = $state(false)
  let addMenuEl: HTMLDivElement | undefined = $state()
  let sheetUploadInputEl = $state<HTMLInputElement | null>(null)
  const actionSheet = getContext<ActionSheetContext>(ACTION_SHEET_KEY)

  function handleClickOutside(e: MouseEvent) {
    if (addMenuEl && !addMenuEl.contains(e.target as Node)) {
      addMenuOpen = false
    }
  }

  $effect(() => {
    if (addMenuOpen) {
      document.addEventListener('click', handleClickOutside, { capture: true })
      return () =>
        document.removeEventListener('click', handleClickOutside, {
          capture: true,
        })
    }
  })

  const activeCollection = $derived(
    navigationStore.activeCollectionId
      ? (collectionsStore.collections.find(
          (c) => c.id === navigationStore.activeCollectionId
        ) ?? null)
      : null
  )

  const activeArchiveGroup = $derived(
    taxonomyGroupById(navigationStore.activeArchiveGroupId)
  )

  function handleUpload(files: File[]) {
    assetsStore.upload(
      files,
      navigationStore.activeProjectId,
      navigationStore.activeFolderId
    )
  }

  function openAddSheet() {
    actionSheet.open({
      title: m.add(),
      items: [
        {
          key: 'upload',
          label: m.upload_files(),
          onSelect: () => sheetUploadInputEl?.click(),
        },
        {
          key: 'members',
          label: m.member_invite(),
          onSelect: () => goto('/library/settings/members'),
        },
        {
          key: 'ingress',
          label: m.add_ingress_source(),
          onSelect: () => goto('/library/settings/ingress'),
        },
        {
          key: 'tags',
          label: m.tags_manage(),
          onSelect: () => goto('/library/settings/tags'),
        },
        {
          key: 'fields',
          label: m.fields_manage(),
          onSelect: () => goto('/library/settings/custom-fields'),
        },
      ],
    })
  }
</script>

<header
  class="flex flex-wrap items-center gap-3 border-b border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3 py-2.5 sm:min-h-16 sm:px-4 xl:flex-nowrap"
>
  <!-- LEFT: breadcrumb / title -->
  <div class="flex min-w-0 shrink items-center gap-2">
    {@render prefix?.()}
    <div class="min-w-0">
      <p class="text-sm font-semibold text-[var(--text-primary)]">
        {projectsStore.activeProjectName ??
          activeCollection?.name ??
          activeArchiveGroup?.label ??
          m.library()}
      </p>
      <p class="truncate text-xs text-[var(--text-muted)]">
        {#if activeCollection}{m.collection_id({
            id: activeCollection.name,
          })}{:else if projectsStore.activeProjectName}{m.project_id({
            name: projectsStore.activeProjectName,
          })}{:else if activeArchiveGroup}{activeArchiveGroup.description}{:else}{m.all_assets()}{/if}
      </p>
    </div>
  </div>

  <!-- CENTER: search -->
  <div
    class="order-3 basis-full xl:order-none xl:mx-auto xl:max-w-xl xl:flex-1"
  >
    <SearchInput
      value={assetsStore.query}
      placeholder={m.search_anything()}
      onchange={(q) => {
        assetsStore.search(q)
      }}
    />
  </div>

  <!-- RIGHT: secondary actions + primary CTA + user -->
  <div class="ml-auto flex shrink-0 items-center gap-1.5 sm:gap-2">
    {#if !viewportStore.isMobile}
      <UndoRedo />
    {/if}

    {#if showShareButton}
      <button
        type="button"
        class="flex min-h-10 min-w-10 items-center justify-center rounded-md text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]"
        onclick={onShareProject}
        title={m.share()}
        aria-label={m.share()}
      >
        <Share2 class="h-4 w-4" />
      </button>
    {/if}

    {#if authStore.role !== 'viewer'}
      <input
        bind:this={sheetUploadInputEl}
        type="file"
        multiple
        class="hidden"
        onchange={(e) => {
          const files = Array.from((e.target as HTMLInputElement).files ?? [])
          handleUpload(files)
          ;(e.target as HTMLInputElement).value = ''
        }}
      />

      <div class="add-btn-group relative flex" bind:this={addMenuEl}>
        <!-- Primary: upload files -->
        <button
          type="button"
          class="add-btn-primary"
          data-upload-trigger
          onclick={() => sheetUploadInputEl?.click()}
        >
          <Plus class="h-4 w-4" />
          {m.add()}
        </button>

        <!-- Chevron toggle -->
        <button
          type="button"
          class="add-btn-chevron"
          onclick={() => {
            if (viewportStore.isTouch) {
              openAddSheet()
              return
            }
            addMenuOpen = !addMenuOpen
          }}
          onkeydown={(e) => {
            if (e.key === 'Escape') addMenuOpen = false
          }}
          aria-label="更多添加方式"
          aria-expanded={addMenuOpen}
          aria-haspopup="menu"
        >
          <ChevronDown
            class="h-4 w-4 transition-transform {addMenuOpen
              ? 'rotate-180'
              : ''}"
          />
        </button>

        <!-- Dropdown -->
        {#if addMenuOpen}
          <div
            role="menu"
            tabindex="-1"
            class="add-dropdown"
            onkeydown={(e) => {
              if (e.key === 'Escape') {
                addMenuOpen = false
              }
            }}
          >
            <!-- Upload -->
            <button
              type="button"
              role="menuitem"
              class="add-dropdown-item w-full"
              onclick={() => {
                sheetUploadInputEl?.click()
                addMenuOpen = false
              }}
            >
              <Upload class="add-dropdown-icon" />
              {m.upload_files()}
            </button>

            <div class="add-dropdown-divider"></div>

            <!-- Workspace actions -->
            <p class="add-dropdown-group-label">工作区</p>
            <a
              href="/library/settings/members"
              role="menuitem"
              class="add-dropdown-item"
              onclick={() => {
                addMenuOpen = false
              }}
            >
              <Users class="add-dropdown-icon" />
              {m.member_invite()}
            </a>
            <a
              href="/library/settings/ingress"
              role="menuitem"
              class="add-dropdown-item"
              onclick={() => {
                addMenuOpen = false
              }}
            >
              <Download class="add-dropdown-icon" />
              {m.add_ingress_source()}
            </a>
            <div class="add-dropdown-divider"></div>

            <!-- Schema actions -->
            <p class="add-dropdown-group-label">分类与字段</p>
            <a
              href="/library/settings/tags"
              role="menuitem"
              class="add-dropdown-item"
              onclick={() => {
                addMenuOpen = false
              }}
            >
              <Tags class="add-dropdown-icon" />
              {m.tags_manage()}
            </a>
            <a
              href="/library/settings/custom-fields"
              role="menuitem"
              class="add-dropdown-item"
              onclick={() => {
                addMenuOpen = false
              }}
            >
              <Settings2 class="add-dropdown-icon" />
              {m.fields_manage()}
            </a>
          </div>
        {/if}
      </div>
    {/if}

    <UserMenu />
  </div>
</header>

<style>
  /* ── Split button shell ── */
  .add-btn-group {
    display: flex;
    border-radius: 8px;
    box-shadow: 0 1px 2px 0 rgb(0 0 0 / 0.08);
  }

  /* ── Primary upload label ── */
  .add-btn-primary {
    display: flex;
    cursor: pointer;
    align-items: center;
    gap: 6px;
    border-radius: 8px 0 0 8px;
    background: var(--accent-cta);
    min-height: 40px;
    padding: 7px 13px;
    font-size: 0.875rem;
    font-weight: 500;
    color: #fff;
    transition:
      background 0.15s ease,
      transform 0.08s ease;
    user-select: none;
  }
  .add-btn-primary:hover {
    background: var(--accent-cta-hover);
  }
  .add-btn-primary:active {
    transform: translateY(1px);
    background: var(--accent-cta-active);
  }

  /* ── Chevron toggle ── */
  .add-btn-chevron {
    display: flex;
    align-items: center;
    border-radius: 0 8px 8px 0;
    border-left: 1px solid rgba(255 255 255 / 0.18);
    background: var(--accent-cta);
    min-height: 40px;
    min-width: 36px;
    justify-content: center;
    padding: 7px 8px;
    color: #fff;
    transition:
      background 0.15s ease,
      transform 0.08s ease;
    cursor: pointer;
  }
  .add-btn-chevron:hover {
    background: var(--accent-cta-hover);
  }
  .add-btn-chevron:active {
    transform: translateY(1px);
  }

  /* ── Dropdown panel ── */
  .add-dropdown {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    z-index: 50;
    min-width: 220px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    padding: 4px 0;
    box-shadow:
      0 0 0 1px rgb(0 0 0 / 0.04),
      0 4px 8px -2px rgb(0 0 0 / 0.12),
      0 16px 32px -8px rgb(0 0 0 / 0.18);
    animation: dropdown-in 0.14s cubic-bezier(0.25, 1, 0.5, 1) both;
    transform-origin: top right;
  }

  :global(.dark) .add-dropdown {
    border-color: var(--border);
    background: var(--bg-surface);
    box-shadow:
      0 0 0 1px rgb(0 0 0 / 0.3),
      0 4px 8px -2px rgb(0 0 0 / 0.4),
      0 16px 32px -8px rgb(0 0 0 / 0.5);
  }

  @keyframes dropdown-in {
    from {
      opacity: 0;
      transform: scale(0.95) translateY(-4px);
    }
    to {
      opacity: 1;
      transform: scale(1) translateY(0);
    }
  }

  /* Group label */
  .add-dropdown-group-label {
    padding: 8px 12px 3px;
    font-size: 0.6875rem;
    font-weight: 500;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--text-muted);
  }

  /* Divider */
  .add-dropdown-divider {
    margin: 4px 0;
    height: 1px;
    background: var(--border-subtle);
  }

  :global(.dark) .add-dropdown-divider {
    background: var(--bg-elevated);
  }

  /* Item row */
  .add-dropdown-item {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 38px;
    padding: 7px 12px;
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--text-primary);
    text-decoration: none;
    transition: background 0.08s ease;
    cursor: pointer;
  }
  .add-dropdown-item:hover {
    background: var(--bg-hover);
  }

  .add-btn-primary:focus-visible,
  .add-btn-chevron:focus-visible,
  .add-dropdown-item:focus-visible {
    outline: 2px solid var(--focus-ring);
    outline-offset: 2px;
  }

  :global(.dark) .add-dropdown-item:hover {
    background: var(--bg-elevated);
  }

  /* Inline icon */
  :global(.add-dropdown-icon) {
    width: 14px;
    height: 14px;
    flex-shrink: 0;
    color: var(--text-muted);
    transition: color 0.08s ease;
  }
  .add-dropdown-item:hover :global(.add-dropdown-icon) {
    color: var(--accent-cta);
  }
</style>
