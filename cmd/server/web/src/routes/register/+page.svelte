<script lang="ts">
  import { goto } from '$app/navigation'
  import { authApi, ApiError } from '$lib/api'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import GeometricBackground from '$lib/components/ui/GeometricBackground.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import Title from '$lib/components/ui/Title.svelte'
  import { m } from '$lib/paraglide/messages'
  import { onMount } from 'svelte'

  let name = $state('')
  let email = $state('')
  let password = $state('')
  let error = $state('')
  let loading = $state(false)

  onMount(() => {
    fetch('/config/auth')
      .then((r) => r.json())
      .then((d) => {
        if (!d.signup_enabled) goto('/login')
      })
      .catch(() => {
        // silent
      })
  })

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    loading = true

    try {
      await authApi.register(name, email, password)
      goto('/library')
    } catch (err) {
      error = err instanceof ApiError ? err.message : m.register_failed()
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>创建账号｜建筑素材中台</title>
</svelte:head>

<div
  class="relative flex min-h-screen items-center justify-center bg-gray-50 dark:bg-gray-950"
>
  <GeometricBackground withStar />

  <div
    class="auth-card z-1 w-full max-w-md space-y-8 rounded-xl bg-white p-8 shadow dark:bg-gray-900"
  >
    <div>
      <Title>创建建筑素材中台账号</Title>
      <Hint>
        {m.already_hve_account_question()}
        <a href="/login" class="text-blue-600 hover:underline">{m.signin()}</a>
      </Hint>
    </div>

    <form onsubmit={handleSubmit} class="space-y-4">
      <Feedback {error} />
      <Input
        id="name"
        type="text"
        label={m.fullname()}
        bind:value={name}
        required
        autocomplete="name"
      />
      <Input
        id="email"
        type="email"
        label={m.email()}
        bind:value={email}
        required
        autocomplete="email"
      />
      <div>
        <Input
          id="password"
          type="password"
          label="密码"
          bind:value={password}
          required
          autocomplete="new-password"
        />
        <p class="mt-1 text-sm text-gray-500">{m.min_8_chars()}</p>
      </div>
      <Button type="submit" {loading} class="w-full"
        >{loading ? m.creating_account() : m.create_account()}</Button
      >
    </form>
  </div>
</div>
