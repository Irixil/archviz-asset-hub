<script lang="ts">
  import { ApiError } from '$lib/api/client'
  import { m } from '$lib/paraglide/messages'
  import Button from '$lib/components/ui/Button.svelte'
  import StatusBadge from '$lib/components/ui/StatusBadge.svelte'
  import { Eye, EyeOff } from '@lucide/svelte'
  import { aiProvidersApi, type ProviderKeyStatus } from '$lib/api/ai_providers'

  interface Props {
    status: ProviderKeyStatus
    isOwner: boolean
  }

  let { status = $bindable(), isOwner }: Props = $props()

  let showForm = $state(false)
  let keyInput = $state('')
  let showKey = $state(false)
  let saving = $state(false)
  let clearing = $state(false)
  let confirmClear = $state(false)
  let testing = $state(false)
  let testResult = $state<'success' | 'failure' | 'error' | null>(null)

  async function refreshStatus() {
    try {
      status = await aiProvidersApi.getAIProviderKeyStatus('openrouter')
    } catch {}
  }

  async function save() {
    if (!keyInput.trim()) return
    saving = true
    testResult = null
    try {
      await aiProvidersApi.setAIProviderKey('openrouter', keyInput.trim())
      keyInput = ''
      showForm = false
      await refreshStatus()
    } finally {
      saving = false
    }
  }

  async function clear() {
    clearing = true
    testResult = null
    try {
      await aiProvidersApi.clearAIProviderKey('openrouter')
      confirmClear = false
      await refreshStatus()
    } finally {
      clearing = false
    }
  }

  async function test() {
    testing = true
    testResult = null
    try {
      await aiProvidersApi.testAIProviderKey('openrouter')
      testResult = 'success'
    } catch (e) {
      testResult =
        e instanceof ApiError && e.status === 422 ? 'failure' : 'error'
    } finally {
      testing = false
    }
  }
</script>

<div
  class="items-top flex gap-5 rounded-xl border border-zinc-200 bg-white p-5 transition-shadow hover:shadow-sm dark:border-zinc-800 dark:bg-gray-900"
>
  <!-- Icon -->
  <div
    class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-zinc-200 bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-800"
  >
    <img src="/openrouter.logo.svg" alt="OpenRouter.ai" class="h-6 w-6" />
  </div>

  <!-- Content -->
  <div class="min-w-0 flex-1">
    <div class="flex flex-wrap items-center gap-2">
      <p class="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
        {m.integrations_openrouter_label()}
      </p>
      {#if status.source === 'workspace'}
        <StatusBadge
          status="healthy"
          text={m.integrations_openrouter_status_workspace()}
        />
      {:else if status.source === 'env'}
        <StatusBadge
          status="healthy"
          text={m.integrations_openrouter_status_env()}
        />
      {:else}
        <StatusBadge
          status="disabled"
          text={m.integrations_openrouter_status_none()}
        />
      {/if}
    </div>

    <p class="mt-1.5 text-sm text-zinc-500 dark:text-zinc-400">
      {m.integrations_openrouter_desc()}
    </p>

    {#if isOwner}
      <div class="mt-4 space-y-3">
        <!-- Actions for workspace key -->
        {#if status.source === 'workspace'}
          {#if !confirmClear}
            <div class="flex flex-wrap items-center gap-3">
              <Button
                variant="secondary"
                size="sm"
                loading={testing}
                onclick={test}
              >
                {m.integrations_openrouter_test()}
              </Button>
              <button
                onclick={() => (confirmClear = true)}
                class="text-sm text-red-500 hover:underline"
              >
                {m.integrations_openrouter_clear()}
              </button>
            </div>
          {:else}
            <div class="flex flex-wrap items-center gap-3">
              <span class="text-sm text-zinc-500 dark:text-zinc-400">
                {m.integrations_openrouter_clear_confirm()}
              </span>
              <button
                onclick={clear}
                disabled={clearing}
                class="text-sm font-semibold text-red-600 hover:underline disabled:opacity-50"
              >
                {m.integrations_openrouter_clear()}
              </button>
              <button
                onclick={() => (confirmClear = false)}
                class="text-sm text-zinc-500 hover:underline"
              >
                Cancel
              </button>
            </div>
          {/if}
        {:else if status.source === 'env'}
          {#if !showForm}
            <button
              onclick={() => (showForm = true)}
              class="text-sm text-indigo-600 hover:underline dark:text-indigo-400"
            >
              {m.integrations_openrouter_override()}
            </button>
          {/if}
        {/if}

        <!-- Key input form (env override or no-key state) -->
        {#if showForm || status.source === 'none'}
          <form
            onsubmit={(e) => {
              e.preventDefault()
              save()
            }}
            class="flex items-center gap-2"
          >
            <div class="relative flex-1">
              <input
                type={showKey ? 'text' : 'password'}
                bind:value={keyInput}
                placeholder={m.integrations_openrouter_key_placeholder()}
                class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 pr-9 text-sm text-zinc-900 focus:border-indigo-400 focus:ring-1 focus:ring-indigo-300 focus:outline-none dark:border-zinc-600 dark:bg-zinc-800 dark:text-zinc-100 dark:placeholder-zinc-500"
              />
              <button
                type="button"
                onclick={() => (showKey = !showKey)}
                class="absolute inset-y-0 right-2 flex items-center text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300"
                tabindex="-1"
              >
                {#if showKey}
                  <EyeOff class="h-3.5 w-3.5" />
                {:else}
                  <Eye class="h-3.5 w-3.5" />
                {/if}
              </button>
            </div>
            <Button
              type="submit"
              variant="primary"
              size="sm"
              loading={saving}
              disabled={!keyInput.trim()}
            >
              {m.integrations_openrouter_save()}
            </Button>
            {#if showForm && status.source === 'env'}
              <button
                type="button"
                onclick={() => {
                  showForm = false
                  keyInput = ''
                }}
                class="text-sm text-zinc-500 hover:underline"
              >
                Cancel
              </button>
            {/if}
          </form>
        {/if}

        <!-- Test result feedback -->
        {#if testResult === 'success'}
          <p class="text-sm font-medium text-green-600 dark:text-green-400">
            ✓ {m.integrations_openrouter_test_success()}
          </p>
        {:else if testResult === 'failure'}
          <p class="text-sm font-medium text-red-600 dark:text-red-400">
            {m.integrations_openrouter_test_failure()}
          </p>
        {:else if testResult === 'error'}
          <p class="text-sm text-zinc-500 dark:text-zinc-400">
            {m.integrations_openrouter_test_error()}
          </p>
        {/if}
      </div>
    {/if}
  </div>

  <!-- External link -->
  <div class="shrink-0">
    <a href="https://openrouter.ai" target="_blank">
      <Button variant="secondary" size="sm">openrouter.ai →</Button>
    </a>
  </div>
</div>
