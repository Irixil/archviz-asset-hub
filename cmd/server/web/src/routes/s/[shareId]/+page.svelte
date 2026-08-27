<script lang="ts">
  import { onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { Lock } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import PoweredByDamask from '$lib/components/PoweredByDamask.svelte'

  import { publicShareApi } from '$lib/api'

  let shareId = $derived(page.params.shareId!)
  let visitorName = $state('')
  let password = $state('')
  let error = $state('')
  let loading = $state(false)
  let checking = $state(true)
  let shareLabel = $state('Shared gallery')
  let requiresPassword = $state(false)

  onMount(async () => {
    visitorName =
      sessionStorage.getItem(`damask_share_visitor_${shareId}`) ?? ''
    try {
      const res = await publicShareApi.getAccess(shareId)
      if (res.ok) {
        const data = await res.json()
        shareLabel = data.label ?? 'Shared gallery'
        requiresPassword = !!data.has_password
      } else {
        error = 'This share link is invalid or has expired.'
      }
    } catch {
      error = 'Failed to load this share.'
    } finally {
      checking = false
    }
  })

  async function handleAccess() {
    if (!visitorName.trim()) {
      error = 'Please enter your name to continue.'
      return
    }
    loading = true
    error = ''
    try {
      const res = await publicShareApi.postAccess(
        shareId,
        visitorName.trim(),
        password || ''
      )
      if (!res.ok) {
        error = requiresPassword
          ? 'Incorrect password or missing name.'
          : 'Unable to access this share.'
        return
      }
      const data = await res.json()
      await cookieStore.set(`share_token_${shareId}`, data.token)
      sessionStorage.setItem(
        `damask_share_visitor_${shareId}`,
        visitorName.trim()
      )
      goto(`/s/${shareId}/view`, { replaceState: true })
    } catch {
      error = 'Please try again.'
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>{shareLabel} — Damask</title>
</svelte:head>

<div
  class="flex min-h-screen flex-col items-center justify-center bg-gray-50 px-4 dark:bg-gray-950"
>
  {#if checking}
    <p class="text-gray-500 dark:text-gray-400">Loading…</p>
  {:else if error && !requiresPassword && !visitorName}
    <div
      class="w-full max-w-md rounded-2xl bg-white p-8 text-center shadow-sm dark:bg-gray-900"
    >
      <p class="text-md text-gray-600 dark:text-gray-400">{error}</p>
    </div>
  {:else}
    <div
      class="w-full max-w-md rounded-2xl bg-white p-8 shadow-sm dark:bg-gray-900"
    >
      <div class="mb-5 flex justify-center">
        <div
          class="flex h-12 w-12 items-center justify-center rounded-xl bg-indigo-100 dark:bg-indigo-900/50"
        >
          <Lock class="h-6 w-6 text-indigo-600 dark:text-indigo-400" />
        </div>
      </div>

      <h1
        class="mb-2 text-center text-2xl font-bold text-gray-900 dark:text-gray-100"
      >
        Access this share
      </h1>
      <p class="text-md mb-6 text-center text-gray-500 dark:text-gray-400">
        {shareLabel}
      </p>

      <form
        onsubmit={(e) => {
          e.preventDefault()
          handleAccess()
        }}
      >
        <Input
          label="Your name"
          placeholder="Enter your name"
          bind:value={visitorName}
        />

        {#if requiresPassword}
          <Input
            label="Password"
            type="password"
            placeholder="Password"
            bind:value={password}
          />
        {/if}

        {#if error}
          <p class="mt-3 text-sm text-red-600 dark:text-red-400">{error}</p>
        {/if}

        <Button type="submit" variant="primary" {loading} class="mt-4 w-full">
          Continue
        </Button>
      </form>
    </div>
  {/if}

  <PoweredByDamask />
</div>
