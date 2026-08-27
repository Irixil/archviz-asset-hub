<script lang="ts">
  import '../auth-page.css'
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { authApi, ApiError } from '$lib/api'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import GeometricBackground from '$lib/components/ui/GeometricBackground.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import { onMount } from 'svelte'
  import { m } from '$lib/paraglide/messages'

  let password = $state('')
  let confirm = $state('')
  let loading = $state(false)
  let error = $state('')

  const token = $derived(page.url.searchParams.get('token') ?? '')

  onMount(() => {
    // if (!token) goto('/forgot-password')
  })

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    if (password !== confirm) {
      error = 'Passwords do not match.'
      return
    }
    loading = true
    error = ''
    try {
      await authApi.resetPassword(token, password)
      goto('/login?reset=1')
    } catch (err) {
      error =
        err instanceof ApiError && err.status === 400
          ? 'This reset link is invalid or has expired.'
          : err instanceof ApiError
            ? err.message
            : 'Could not reset password.'
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>{m.choose_new_password()} — Damask</title>
</svelte:head>

<div class="auth-root">
  <GeometricBackground />

  <div class="auth-content w-full md:max-w-lg">
    <div class="auth-card">
      <h1 class="auth-heading">{m.choose_new_password()}</h1>
      <p class="auth-body">
        Pick something strong, you won't need to change it again.
      </p>

      <form onsubmit={submit} class="rp-form">
        <Feedback {error} class="mx-0" />
        <Input
          label="New password"
          type="password"
          bind:value={password}
          required
          autocomplete="new-password"
        />
        <Input
          label="Confirm new password"
          type="password"
          bind:value={confirm}
          required
          autocomplete="new-password"
        />
        <Button type="submit" {loading} class="w-full">Set new password</Button>
      </form>

      <a href="/login" class="rp-back-link">Back to sign in</a>
    </div>

    <p class="auth-footer">Damask DAM</p>
  </div>
</div>

<style>
  .rp-form {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    margin-bottom: 1.25rem;
  }

  .rp-back-link {
    display: block;
    text-align: center;
    font-size: 0.75rem;
    color: var(--color-gray-500);
    text-decoration: none;
    transition: color 0.15s ease;
  }
  .rp-back-link:hover {
    color: #d5d1cd;
  }
  .rp-back-link:focus-visible {
    outline: 2px solid #d5d1cd;
    outline-offset: 2px;
    border-radius: 2px;
  }

  /* Input + button palette overrides for the dark card */
  :global(
    .auth-card input[type='email'],
    .auth-card input[type='password'],
    .auth-card input[type='text']
  ) {
    background: #1a1730;
    border-color: #2a2240;
    color: var(--color-gray-300);
    caret-color: #d5d1cd;
  }
  :global(.auth-card input::placeholder) {
    color: var(--color-gray-500);
  }
  :global(.auth-card input:focus) {
    border-color: #d5d1cd55;
    box-shadow: 0 0 0 2px #d5d1cd22;
    outline: none;
  }

  @media (prefers-reduced-motion: no-preference) {
    .rp-back-link {
      transition: color 0.15s ease;
    }
  }
</style>
