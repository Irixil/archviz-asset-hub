<script lang="ts">
  import '../auth-page.css'
  import { authApi, ApiError } from '$lib/api'
  import { CircleCheck } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import GeometricBackground from '$lib/components/ui/GeometricBackground.svelte'

  let email = $state('')
  let loading = $state(false)
  let sent = $state(false)
  let error = $state('')

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    loading = true
    error = ''
    try {
      await authApi.forgotPassword(email)
      sent = true
    } catch (err) {
      error =
        err instanceof ApiError ? err.message : 'Could not send reset link.'
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>Reset your password — Damask</title>
</svelte:head>

<div class="auth-root">
  <GeometricBackground />

  <div class="auth-content w-full md:max-w-lg">
    <div class="auth-card">
      <h1 class="auth-heading">Reset your password</h1>

      {#if sent}
        <div class="fp-sent">
          <CircleCheck size={16} class="fp-sent-icon" />
          <p>Check your email for a reset link.</p>
        </div>
        <p class="auth-body">
          If an account exists for <strong>{email}</strong>, you'll receive
          instructions shortly.
        </p>
        <a href="/login" class="fp-back-link">Back to sign in</a>
      {:else}
        <p class="auth-body">
          Enter your email and we'll send you a reset link.
        </p>
        <form onsubmit={submit} class="fp-form">
          <Feedback {error} class="mx-0" />
          <Input
            label="Email"
            type="email"
            bind:value={email}
            required
            autocomplete="email"
          />
          <Button type="submit" {loading} class="w-full">Send reset link</Button
          >
        </form>
        <a href="/login" class="fp-back-link">Back to sign in</a>
      {/if}
    </div>

    <p class="auth-footer">Damask DAM</p>
  </div>
</div>

<style>
  .fp-form {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    margin-bottom: 1.25rem;
  }

  /* Sent confirmation */
  .fp-sent {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.75rem;
    color: #d5d1cd;
    font-size: 0.8125rem;
    font-weight: 500;
  }

  :global(.fp-sent-icon) {
    flex-shrink: 0;
    color: #d5d1cd;
  }

  .fp-back-link {
    display: block;
    text-align: center;
    font-size: 0.75rem;
    color: var(--color-gray-500);
    text-decoration: none;
    transition: color 0.15s ease;
  }
  .fp-back-link:hover {
    color: #d5d1cd;
  }
  .fp-back-link:focus-visible {
    outline: 2px solid #d5d1cd;
    outline-offset: 2px;
    border-radius: 2px;
  }

  @media (prefers-reduced-motion: no-preference) {
    .fp-back-link {
      transition: color 0.15s ease;
    }
  }
</style>
