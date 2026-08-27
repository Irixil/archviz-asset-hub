<script lang="ts">
  import { m } from '$lib/paraglide/messages'

  interface Props {
    provider: 'password' | 'oidc' | 'google' | 'canva'
    label: string
    linked: boolean
    email?: string
    connectHref?: string
    onDisconnect?: () => void
    disconnectError?: string
  }

  let {
    provider,
    label,
    linked,
    email,
    connectHref,
    onDisconnect,
    disconnectError,
  }: Props = $props()
</script>

<div
  class="flex items-center justify-between gap-4 border-b border-[var(--border-subtle)] py-4 last:border-0"
>
  <div class="min-w-0">
    <p class="font-medium text-[var(--text-primary)]">{label}</p>
    {#if linked && email}
      <p class="text-sm text-[var(--text-muted)]">
        {m.settings_auth_connected_as({ email })}
      </p>
    {:else if linked}
      <p class="text-sm text-[var(--text-muted)]">
        {m.settings_auth_connected_as({ email: label })}
      </p>
    {:else if provider !== 'password'}
      <p class="text-sm text-[var(--text-muted)]">—</p>
    {/if}
    {#if disconnectError}
      <p class="mt-0.5 text-sm text-red-500">{disconnectError}</p>
    {/if}
  </div>

  <div class="shrink-0">
    {#if provider === 'password'}
      <!-- password row: no connect/disconnect -->
    {:else if linked}
      <button
        type="button"
        onclick={onDisconnect}
        class="text-sm text-red-500 hover:underline"
        >{m.settings_auth_disconnect()}</button
      >
    {:else if connectHref}
      <a href={connectHref} class="text-sm text-[var(--accent)] hover:underline"
        >{m.settings_auth_connect()}</a
      >
    {/if}
  </div>
</div>
