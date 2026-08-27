<script lang="ts">
  import { m } from '$lib/paraglide/messages'
  import Button from '$lib/components/ui/Button.svelte'
  import StatusBadge from '$lib/components/ui/StatusBadge.svelte'
  import GoogleIcon from '$lib/components/ui/GoogleIcon.svelte'
  import { X } from '@lucide/svelte'

  interface Props {
    provider: string
    label: string
    description: string
    connected: boolean
    providerEmail?: string
    connectHref: string
    onDisconnect?: () => void
    setupSourceHref?: string
    needsReauth?: boolean
  }
  let {
    provider,
    label,
    description,
    connected,
    providerEmail,
    connectHref,
    onDisconnect,
    setupSourceHref,
    needsReauth = false,
  }: Props = $props()

  let confirming = $state(false)
</script>

<div
  class="items-top flex gap-5 rounded-xl border border-zinc-200 bg-white p-5 transition-shadow hover:shadow-sm dark:border-zinc-800 dark:bg-gray-900"
>
  <!-- Provider icon -->
  <div
    class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-zinc-200 bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-800"
  >
    {#if provider === 'google'}
      <GoogleIcon size={8} />
    {:else if provider === 'canva'}
      <img src="/canva.logo.svg" width="30" alt="canva logo" />
    {:else}
      <span class="text-sm font-medium text-zinc-400">{label[0]}</span>
    {/if}
  </div>

  <!-- Label + description -->
  <div class="min-w-0 flex-1">
    <div class="flex flex-wrap items-center gap-2">
      <p class="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
        {label}
      </p>
      {#if connected}
        <StatusBadge
          status="healthy"
          text={m.integrations_connected_as({ email: providerEmail ?? label })}
        />
      {:else}
        <StatusBadge status="disabled" text={m.integrations_not_connected()} />
      {/if}
    </div>
    <p class="mt-1.5 truncate text-sm text-zinc-500 dark:text-zinc-400">
      {description}
    </p>

    {#if connected && needsReauth}
      <div
        class="mt-3 flex items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-800/50 dark:bg-amber-900/20 dark:text-amber-300"
      >
        <span
          >Reconnect to enable Google Drive exports (updated permissions
          required).</span
        >
        <a
          href={connectHref}
          class="ml-auto shrink-0 font-medium underline hover:no-underline"
          >Reconnect</a
        >
      </div>
    {/if}

    {#if connected && (setupSourceHref || onDisconnect)}
      <div class="mt-4 flex flex-wrap items-center gap-3">
        {#if !confirming}
          <button
            onclick={() => (confirming = true)}
            class="flex items-center gap-1.5 text-sm text-red-500 hover:underline"
            ><X class="h-3 w-3" /> {m.integrations_disconnect()}</button
          >
        {:else}
          <span class="text-sm text-zinc-500 dark:text-zinc-400">
            {m.integrations_disconnect_confirm({ provider: label })}
          </span>
          <button
            onclick={() => {
              confirming = false
              onDisconnect?.()
            }}
            class="text-sm font-semibold text-red-600 hover:underline"
            >{m.integrations_disconnect()}</button
          >
          <button
            onclick={() => (confirming = false)}
            class="text-sm text-zinc-500 hover:underline">Cancel</button
          >
        {/if}
      </div>
    {/if}
  </div>

  <!-- Action button -->
  <div class="shrink-0">
    {#if connected}
      {#if setupSourceHref}
        <a href={setupSourceHref}>
          <Button variant="secondary" size="sm"
            >{m.integrations_setup_source().replace(' →', '')}</Button
          >
        </a>
      {/if}
    {:else}
      <a href={connectHref}>
        <Button variant="outline" size="sm"
          >{m.integrations_connect({ provider: label })}</Button
        >
      </a>
    {/if}
  </div>
</div>
