<script lang="ts">
  import '../auth-page.css'
  import { goto } from '$app/navigation'
  import { page } from '$app/stores'
  import { workspaceApi, ApiError } from '$lib/api'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import GeometricBackground from '$lib/components/ui/GeometricBackground.svelte'
  import { m } from '$lib/paraglide/messages'

  const token = $page.url.searchParams.get('token') ?? ''

  let name = $state('')
  let password = $state('')
  let error = $state('')
  let loading = $state(false)

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    loading = true

    try {
      await workspaceApi.acceptInvite(token, name, password)
      goto('/library')
    } catch (err) {
      error = err instanceof ApiError ? err.message : m.cannot_accept_invite()
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>{m.accept_invite()} — Damask</title>
</svelte:head>

<div class="auth-root">
  <GeometricBackground withStar />

  <div class="auth-content w-full md:max-w-lg">
    <div class="auth-card">
      {#if token.trim() === ''}
        <h1 class="auth-heading">{m.invalid_invite()}</h1>
        <p class="auth-body">{m.invite_missing_token()}</p>
      {:else}
        <h1 class="auth-heading">{m.accept_invite()}</h1>
        <p class="auth-body">{m.create_account_join_workspace()}</p>

        <form onsubmit={handleSubmit} class="invite-form">
          <Feedback {error} class="mx-0" />
          <Input
            id="name"
            type="text"
            label={m.name()}
            bind:value={name}
            required
            autocomplete="name"
          />
          <Input
            id="password"
            type="password"
            label={m.password()}
            bind:value={password}
            required
            autocomplete="new-password"
            placeholder={m.min_8_chars()}
          />
          <Button type="submit" {loading} class="w-full"
            >{loading ? m.joining() : m.join_workspace()}</Button
          >
        </form>
      {/if}
    </div>

    <p class="auth-footer">Damask DAM</p>
  </div>
</div>

<style>
  .invite-form {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  :global(.auth-card input:focus) {
    border-color: #d5d1cd55;
    box-shadow: 0 0 0 2px #d5d1cd22;
    outline: none;
  }
</style>
