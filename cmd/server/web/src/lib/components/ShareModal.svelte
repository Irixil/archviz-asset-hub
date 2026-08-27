<script lang="ts">
  import { EXPIRY_OPTIONS, sharesStore } from '$lib/stores/shares.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import type { Share } from '$lib/api'
  import Modal from '$lib/components/ui/Modal.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import {
    Check,
    Copy,
    Globe,
    File,
    Link,
    Lock,
    MessageSquare,
    Download,
    Calendar,
    Trash2,
    RefreshCw,
  } from '@lucide/svelte'
  import { m } from '$lib/paraglide/messages'

  interface TargetOption {
    type: 'asset' | 'project'
    id: string
    label: string
    assetCount?: number
  }

  interface Props {
    open?: boolean
    /** Initial target — if set, the target picker is pre-filled (and locked if only one option) */
    targets: TargetOption[]
    onclose?: () => void
  }

  let { open = $bindable(false), targets, onclose }: Props = $props()

  // ---- form state ----
  // svelte-ignore state_referenced_locally
  let selectedTargetIdx = $state(targets.length > 1 ? 1 : 0) // default to first (project) if both
  let label = $state('')
  let password = $state('')
  let passwordEnabled = $state(false)
  let allowDownload = $state(true)
  let allowComments = $state(true)
  let expiryDays = $state<number | null>(30)
  let submitting = $state(false)
  let createdShare = $state<Share | null>(null)
  let copied = $state(false)

  const selectedTarget = $derived(targets[selectedTargetIdx] ?? targets[0])

  // Check if an active (non-revoked, non-expired) share already exists for this target
  const existingShare = $derived(
    selectedTarget
      ? (sharesStore
          .forTarget(selectedTarget.type, selectedTarget.id)
          .find((s) => !s.is_expired) ?? null)
      : null
  )

  function newPassword(length = 8) {
    const chars =
      'ABCDEFGHJKLMNOPQRSTUVWXYZabcdefghjklmnopqrstuwxyzABCDEFGHJKLMNOPQRSTUVWXYZabcdefghjklmnopqrstuwxyz23456789!@#$%&*+:;?./-='
    let result = ''
    for (let i = 0; i < length; i++) {
      result += chars.charAt(Math.floor(Math.random() * chars.length))
    }
    return result
  }

  function reset() {
    label = ''
    password = ''
    passwordEnabled = false
    allowDownload = true
    allowComments = true
    expiryDays = 7
    submitting = false
    createdShare = null
    copied = false
    selectedTargetIdx = targets.length > 1 ? 1 : 0
  }

  function handleClose() {
    reset()
    onclose?.()
    open = false
  }

  async function handleSubmit() {
    if (!selectedTarget) return
    submitting = true
    const share = await sharesStore.create({
      label: label.trim() || undefined,
      target_type: selectedTarget.type,
      target_id: selectedTarget.id,
      password: passwordEnabled && password ? password : undefined,
      expires_in_days: expiryDays ?? undefined,
      allow_comments: allowComments,
      allow_download: allowDownload,
    })
    submitting = false
    if (share) createdShare = share
  }

  async function handleRevoke(share: Share) {
    await sharesStore.revoke(share.id)
  }

  function copyLink(url: string) {
    navigator.clipboard
      .writeText(url)
      .then(() => {
        copied = true
        toastStore.show('Link copied!')
        setTimeout(() => {
          copied = false
        }, 2000)
      })
      .catch(() => {
        toastStore.show('Could not copy link', 'error')
      })
  }

  // Reset created state when target changes
  $effect(() => {
    if (selectedTargetIdx !== undefined) {
      createdShare = null
      copied = false
    }
  })
</script>

<Modal bind:open onclose={handleClose}>
  <div class="p-6">
    <!-- Header -->
    <div class="mb-6 flex items-center gap-3">
      <div
        class="flex h-9 w-9 items-center justify-center rounded-xl bg-indigo-100 dark:bg-indigo-900/40"
      >
        <Link class="h-4 w-4 text-indigo-600 dark:text-indigo-400" />
      </div>
      <h2 class="text-xl font-semibold text-gray-900 dark:text-gray-50">
        {m.share_create_short()}
      </h2>
    </div>

    {#if createdShare || existingShare}
      {@const share = createdShare ?? existingShare!}
      <!-- Success / Existing share view -->
      <div class="space-y-4">
        <p class="text-md text-gray-600 dark:text-gray-400">
          {createdShare ? m.share_ready() : m.share_already_exists()}
        </p>

        <!-- Link copy box -->
        <div
          class="flex items-center gap-2 rounded-xl border border-indigo-200 bg-indigo-50 p-3 dark:border-indigo-800/60 dark:bg-indigo-900/20"
        >
          <span
            class="text-md min-w-0 flex-1 truncate font-mono text-indigo-700 dark:text-indigo-300"
          >
            {share.public_url}
          </span>
          <button
            type="button"
            class="shrink-0 rounded-lg p-1.5 text-indigo-600 transition-colors hover:bg-indigo-100 dark:text-indigo-400 dark:hover:bg-indigo-800/40"
            onclick={() => copyLink(share.public_url)}
            aria-label="Copy link"
          >
            {#if copied}
              <Check class="h-4 w-4" />
            {:else}
              <Copy class="h-4 w-4" />
            {/if}
          </button>
        </div>

        <!-- Share meta pills -->
        <div class="flex flex-wrap gap-2 text-sm">
          {#if share.has_password}
            <span
              class="inline-flex items-center gap-1 rounded-full bg-amber-100 px-2.5 py-1 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400"
            >
              <Lock class="h-3 w-3" />
              {m.pass_protected()}
            </span>
          {/if}
          {#if share.expires_at}
            <span
              class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-gray-600 dark:bg-gray-800 dark:text-gray-400"
            >
              <Calendar class="h-3 w-3" />
              {m.expires_on({
                date: new Date(share.expires_at).toLocaleDateString(),
              })}
            </span>
          {:else}
            <span
              class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-gray-600 dark:bg-gray-800 dark:text-gray-400"
            >
              <Calendar class="h-3 w-3" />
              {m.no_expiry()}
            </span>
          {/if}
          {#if share.allow_download}
            <span
              class="inline-flex items-center gap-1 rounded-full bg-green-100 px-2.5 py-1 text-green-700 dark:bg-green-900/30 dark:text-green-400"
            >
              <Download class="h-3 w-3" />
              {m.downloads_enabled()}
            </span>
          {/if}
          {#if share.allow_comments}
            <span
              class="inline-flex items-center gap-1 rounded-full bg-blue-100 px-2.5 py-1 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
            >
              <MessageSquare class="h-3 w-3" />
              {m.comments_enabled()}
            </span>
          {/if}
          <span
            class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-gray-600 dark:bg-gray-800 dark:text-gray-400"
          >
            {share.view_count} view{share.view_count !== 1 ? 's' : ''}
          </span>
        </div>

        <div class="flex gap-2 pt-2">
          <Button
            variant="primary"
            class="flex-1"
            onclick={() => copyLink(share.public_url)}
          >
            {#snippet icon()}<Copy class="h-3.5 w-3.5" />{/snippet}
            Copy Link
          </Button>
          <Button
            variant="danger"
            onclick={() =>
              handleRevoke(share).then(() => {
                createdShare = null
              })}
          >
            {#snippet icon()}<Trash2 class="h-3.5 w-3.5" />{/snippet}
            Revoke
          </Button>
          <Button variant="secondary" onclick={handleClose}>Close</Button>
        </div>
      </div>
    {:else}
      <!-- Create form -->
      <form
        onsubmit={(e) => {
          e.preventDefault()
          handleSubmit()
        }}
        class="space-y-5"
      >
        <!-- Target picker -->
        {#if targets.length > 1}
          <div>
            <p
              class="text-md mb-2 font-medium text-gray-700 dark:text-gray-300"
            >
              {m.what_to_share()}
            </p>
            <div class="grid grid-cols-2 gap-3">
              {#each targets as t, i}
                <button
                  type="button"
                  class="flex flex-col items-start gap-1 rounded-xl border p-4 text-left transition-colors
                    {selectedTargetIdx === i
                    ? 'border-indigo-400 bg-indigo-50 dark:border-indigo-600 dark:bg-indigo-900/20'
                    : 'border-gray-200 hover:border-gray-300 dark:border-gray-700 dark:hover:border-gray-600'}"
                  onclick={() => {
                    selectedTargetIdx = i
                  }}
                >
                  {#if t.type === 'asset'}
                    <File
                      class="h-5 w-5 {selectedTargetIdx === i
                        ? 'text-indigo-600 dark:text-indigo-400'
                        : 'text-gray-400'}"
                    />
                  {:else}
                    <Globe
                      class="h-5 w-5 {selectedTargetIdx === i
                        ? 'text-indigo-600 dark:text-indigo-400'
                        : 'text-gray-400'}"
                    />
                  {/if}
                  <span
                    class="text-md font-medium {selectedTargetIdx === i
                      ? 'text-indigo-700 dark:text-indigo-300'
                      : 'text-gray-700 dark:text-gray-300'}"
                  >
                    {t.label}
                  </span>
                  {#if t.assetCount !== undefined}
                    <span class="text-sm text-gray-400"
                      >{t.assetCount} asset{t.assetCount !== 1 ? 's' : ''}</span
                    >
                  {/if}
                </button>
              {/each}
            </div>
          </div>
        {/if}

        <!-- Label -->
        <Input
          bind:value={label}
          label="Description"
          placeholder="Add a note for reviewers…"
          id="share-label"
        />

        <!-- Link Settings -->
        <div>
          <p class="text-md mb-3 font-medium text-gray-700 dark:text-gray-300">
            Link Settings
          </p>
          <div
            class="divide-y divide-gray-100 rounded-xl border border-gray-200 dark:divide-gray-800 dark:border-gray-700"
          >
            <!-- Require Password -->
            <div class="flex items-center gap-3 px-4 py-3">
              <div class="flex h-8 w-8 items-center justify-center">
                <Lock class="h-4 w-4 text-gray-500 dark:text-gray-400" />
              </div>
              <div class="flex-1">
                <p class="text-md font-medium text-gray-800 dark:text-gray-200">
                  Require Password
                </p>
                <p class="text-sm text-gray-400 dark:text-gray-500">
                  Only people with the password can view
                </p>
              </div>
              <button
                type="button"
                aria-label="Toggle password protection"
                class="relative inline-flex h-6 w-11 shrink-0 rounded-full transition-colors
                  {passwordEnabled
                  ? 'bg-indigo-600'
                  : 'bg-gray-200 dark:bg-gray-700'}"
                onclick={() => {
                  passwordEnabled = !passwordEnabled
                }}
                aria-checked={passwordEnabled}
                role="switch"
              >
                <span
                  class="absolute top-0.5 left-0.5 h-5 w-5 rounded-full bg-white shadow transition-transform
                  {passwordEnabled ? 'translate-x-5' : 'translate-x-0'}"
                ></span>
              </button>
            </div>
            {#if passwordEnabled}
              <div class="flex px-4 pt-2 pb-3">
                <Input
                  bind:value={password}
                  type="text"
                  placeholder="Enter password…"
                  id="share-password"
                  autocomplete="new-password"
                />
                <Button
                  variant="secondary"
                  class="ml-2 whitespace-nowrap"
                  onclick={() => {
                    password = newPassword()
                  }}><RefreshCw class="h-4 w-4" /></Button
                >
              </div>
            {/if}

            <!-- Allow Downloads -->
            <div class="flex items-center gap-3 px-4 py-3">
              <div class="flex h-8 w-8 items-center justify-center">
                <Download class="h-4 w-4 text-gray-500 dark:text-gray-400" />
              </div>
              <div class="flex-1">
                <p class="text-md font-medium text-gray-800 dark:text-gray-200">
                  Allow Downloads
                </p>
                <p class="text-sm text-gray-400 dark:text-gray-500">
                  Viewers can download original files
                </p>
              </div>
              <button
                type="button"
                aria-label="Toggle allow downloads"
                class="relative inline-flex h-6 w-11 shrink-0 rounded-full transition-colors
                  {allowDownload
                  ? 'bg-indigo-600'
                  : 'bg-gray-200 dark:bg-gray-700'}"
                onclick={() => {
                  allowDownload = !allowDownload
                }}
                aria-checked={allowDownload}
                role="switch"
              >
                <span
                  class="absolute top-0.5 left-0.5 h-5 w-5 rounded-full bg-white shadow transition-transform
                  {allowDownload ? 'translate-x-5' : 'translate-x-0'}"
                ></span>
              </button>
            </div>

            <!-- Allow Comments -->
            <div class="flex items-center gap-3 px-4 py-3">
              <div class="flex h-8 w-8 items-center justify-center">
                <MessageSquare
                  class="h-4 w-4 text-gray-500 dark:text-gray-400"
                />
              </div>
              <div class="flex-1">
                <p class="text-md font-medium text-gray-800 dark:text-gray-200">
                  Allow Comments
                </p>
                <p class="text-sm text-gray-400 dark:text-gray-500">
                  Viewers can leave feedback on assets
                </p>
              </div>
              <button
                type="button"
                aria-label="Toggle allow comments"
                class="relative inline-flex h-6 w-11 shrink-0 rounded-full transition-colors
                  {allowComments
                  ? 'bg-indigo-600'
                  : 'bg-gray-200 dark:bg-gray-700'}"
                onclick={() => {
                  allowComments = !allowComments
                }}
                aria-checked={allowComments}
                role="switch"
              >
                <span
                  class="absolute top-0.5 left-0.5 h-5 w-5 rounded-full bg-white shadow transition-transform
                  {allowComments ? 'translate-x-5' : 'translate-x-0'}"
                ></span>
              </button>
            </div>

            <!-- Link Expiration -->
            <div class="flex items-center gap-3 px-4 py-3">
              <div class="flex h-8 w-8 items-center justify-center">
                <Calendar class="h-4 w-4 text-gray-500 dark:text-gray-400" />
              </div>
              <div class="flex-1">
                <p class="text-md font-medium text-gray-800 dark:text-gray-200">
                  Link Expiration
                </p>
              </div>
              <select
                class="text-md rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-gray-700 focus:border-indigo-400 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
                value={expiryDays === null ? 'null' : String(expiryDays)}
                onchange={(e) => {
                  const v = (e.target as HTMLSelectElement).value
                  expiryDays = v === 'null' ? null : Number(v)
                }}
              >
                {#each EXPIRY_OPTIONS as opt}
                  <option
                    value={opt.value === null ? 'null' : String(opt.value)}
                    >{opt.label}</option
                  >
                {/each}
              </select>
            </div>
          </div>
        </div>

        <!-- Submit -->
        <Button
          type="submit"
          variant="primary"
          loading={submitting}
          class="w-full"
        >
          {#snippet icon()}<Link class="h-3.5 w-3.5" />{/snippet}
          Create Share Link
        </Button>
      </form>
    {/if}
  </div>
</Modal>
