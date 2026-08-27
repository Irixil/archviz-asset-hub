<script lang="ts">
  import { Mail } from '@lucide/svelte'
  import type { IngressSource, Folder } from '$lib/api'
  import { configStore } from '$lib/stores/config.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import Hint from '../ui/Hint.svelte'
  import ButtonCopy from '../ui/ButtonCopy.svelte'
  import Button from '../ui/Button.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    source: IngressSource
    folders?: Folder[]
  }

  let { source, folders = [] }: Props = $props()

  // The ingest address is the public_token of the email_api source
  const baseAddress = $derived(
    source.public_token
      ? `${source.public_token}@${configStore.state.mailHost}`
      : null
  )

  let copied = $state(false)
  let copiedFolderId = $state<string | null>(null)

  async function copyAddress(address: string, folderId?: string) {
    try {
      await navigator.clipboard.writeText(address)
      if (folderId) {
        copiedFolderId = folderId
        setTimeout(() => {
          copiedFolderId = null
        }, 2000)
      } else {
        copied = true
        setTimeout(() => {
          copied = false
        }, 2000)
      }
      toastStore.show('Address copied!')
    } catch {
      toastStore.show('Could not copy', 'error')
    }
  }

  function openMailto() {
    if (!baseAddress) return
    const subject = encodeURIComponent('Damask ingest test')
    const body = encodeURIComponent(
      'Attach a file to this email to test your ingest source.'
    )
    window.open(`mailto:${baseAddress}?subject=${subject}&body=${body}`)
  }

  const foldersWithSlug = $derived(folders.filter((f) => f.slug))
</script>

<div class="space-y-5">
  <div
    class="rounded-xl border border-indigo-100 bg-indigo-50/60 p-5 dark:border-indigo-900/40 dark:bg-indigo-900/20"
  >
    <p
      class="mb-3 text-sm font-semibold tracking-widest text-indigo-500 uppercase dark:text-indigo-400"
    >
      {m.ingress_mail_yours()}
    </p>

    {#if baseAddress}
      <div
        class="flex items-center gap-2 rounded-lg border border-indigo-200 bg-white px-3 py-2 dark:border-indigo-800 dark:bg-gray-900"
      >
        <Mail class="h-4 w-4 shrink-0 text-indigo-400" />
        <span
          class="text-md flex-1 font-mono break-all text-gray-800 dark:text-gray-200"
          >{baseAddress}</span
        >
        <ButtonCopy onclick={() => copyAddress(baseAddress!)} {copied} />
      </div>
    {:else}
      <Hint class="italic">{m.ingress_mail_not_available()}</Hint>
    {/if}

    <Hint class="mt-3 text-sm">
      {m.ingress_mail_help_attachments()}
    </Hint>
  </div>

  {#if baseAddress && foldersWithSlug.length > 0}
    <div
      class="rounded-xl border border-gray-200 bg-gray-50 p-5 dark:border-gray-700 dark:bg-gray-800/40"
    >
      <p
        class="mb-1 text-sm font-semibold tracking-widest text-gray-500 uppercase dark:text-gray-400"
      >
        {m.folder_routing()}
      </p>
      <p class="mb-3 text-sm text-gray-500 dark:text-gray-400">
        Add <span class="font-mono">+folder-name</span> to route attachments to a
        specific folder.
      </p>
      <div class="space-y-1.5">
        {#each foldersWithSlug as folder (folder.id)}
          {@const folderAddress = `${source.public_token}+${folder.slug}@${configStore.state.mailHost}`}
          <div
            class="flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 py-2 dark:border-gray-700 dark:bg-gray-900"
          >
            <span
              class="w-32 shrink-0 truncate text-sm text-gray-500 dark:text-gray-400"
              >{folder.name}</span
            >
            <span
              class="flex-1 font-mono text-sm break-all text-gray-700 dark:text-gray-300"
              >{folderAddress}</span
            >
            <ButtonCopy
              onclick={() => copyAddress(folderAddress, folder.id)}
              copied={copiedFolderId === folder.id}
            />
          </div>
        {/each}
      </div>
      <Hint class="mt-3 text-sm">
        {m.folder_routing_help_default()}
      </Hint>
    </div>
  {/if}

  {#if baseAddress}
    <Button
      variant="outline"
      size="md"
      class="w-full justify-center"
      onclick={openMailto}
    >
      <Mail class="mr-1 h-4 w-4" />
      {m.send_test_email()}
    </Button>
  {/if}
</div>
