<script lang="ts">
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { authApi, ApiError } from '$lib/api'
  import { configStore } from '$lib/stores/config.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import Title from '$lib/components/ui/Title.svelte'
  import OAuthButton from '$lib/components/OAuthButton.svelte'
  import { m } from '$lib/paraglide/messages'
  import GeometricBackground from '$lib/components/ui/GeometricBackground.svelte'

  interface AuthConfig {
    password_auth: boolean
    signup_enabled: boolean
    oidc_enabled: boolean
    oidc_label: string
    google_enabled: boolean
    canva_enabled: boolean
  }

  let authConfig = $state<AuthConfig>({
    password_auth: true,
    signup_enabled: true,
    oidc_enabled: false,
    oidc_label: '使用公司账号登录',
    google_enabled: false,
    canva_enabled: false,
  })

  $effect(() => {
    fetch('/config/auth')
      .then((r) => r.json())
      .then((d) => {
        authConfig = d
      })
      .catch(() => {
        authConfig = { ...authConfig, password_auth: true }
      })
  })

  const ssoErrorMessages: Record<string, string> = {
    oidc_error: '公司账号服务返回错误，请重试。',
    oidc_exchange: '无法完成登录，请重试。',
    email_not_verified: '公司账号中的邮箱尚未验证。',
  }

  const ssoError = $derived(
    (() => {
      const e = page.url.searchParams.get('error')
      return e ? (ssoErrorMessages[e] ?? m.login_failed()) : ''
    })()
  )

  const resetSuccess = $derived(page.url.searchParams.get('reset') === '1')
  const accountDeleted = $derived(
    page.url.searchParams.get('account_deleted') === '1'
  )

  const hasSSOProviders = $derived(
    authConfig.oidc_enabled ||
      authConfig.google_enabled ||
      authConfig.canva_enabled
  )

  let email = $state('')
  let password = $state('')
  let error = $state('')
  let loading = $state(false)
  let demoLoading = $state(false)

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    loading = true
    try {
      await authApi.login(email, password)
      goto('/library')
    } catch (err) {
      error = err instanceof ApiError ? err.message : m.login_failed()
    } finally {
      loading = false
    }
  }

  async function handleDemo() {
    demoLoading = true
    error = ''
    try {
      await authApi.demoSession()
      goto('/library')
    } catch (err) {
      error = err instanceof ApiError ? err.message : m.cannot_start_demo()
    } finally {
      demoLoading = false
    }
  }
</script>

<svelte:head>
  <title>登录｜建筑素材中台</title>
</svelte:head>

<div
  class="relative flex min-h-screen items-center justify-center bg-gray-50 dark:bg-gray-950"
>
  <GeometricBackground withStar />

  <div
    class="auth-card z-1 w-full space-y-8 rounded-xl bg-white p-8 shadow md:max-w-lg dark:bg-gray-900"
  >
    <div>
      <Title>建筑素材中台</Title>
      <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
        登录后搜索、整理和复用公司项目素材
      </p>
      {#if authConfig.signup_enabled}
        <Hint>
          首次使用？
          <a href="/register" class="text-blue-600 hover:underline"
            >创建本地账号</a
          >
        </Hint>
      {/if}
    </div>

    {#if ssoError}
      <Feedback error={ssoError} />
    {/if}

    {#if resetSuccess}
      <Feedback success="密码已更新，现在可以登录。" />
    {/if}

    {#if accountDeleted}
      <Feedback success="账号已删除。" />
    {/if}

    {#if hasSSOProviders}
      <div class="space-y-2">
        {#if authConfig.oidc_enabled}
          <OAuthButton
            provider="oidc"
            label={authConfig.oidc_label}
            href="/auth/oidc/login"
          />
        {/if}
        {#if authConfig.google_enabled}
          <OAuthButton
            provider="google"
            label={m.auth_sso_google()}
            href="/auth/google/login"
          />
        {/if}
        {#if authConfig.canva_enabled}
          <OAuthButton
            provider="canva"
            label={m.auth_sso_canva()}
            href="/auth/canva/login"
          />
        {/if}
      </div>
    {/if}

    {#if hasSSOProviders && authConfig.password_auth}
      <div class="relative flex items-center gap-3">
        <div class="flex-1 border-t border-zinc-200 dark:border-zinc-700"></div>
        <span class="text-xs text-zinc-400 dark:text-zinc-500">或</span>
        <div class="flex-1 border-t border-zinc-200 dark:border-zinc-700"></div>
      </div>
    {/if}

    {#if authConfig.password_auth}
      <form onsubmit={handleSubmit} class="space-y-4">
        <Feedback {error} />
        <Input
          id="email"
          type="email"
          label={m.email()}
          bind:value={email}
          required
          autocomplete="email"
        />
        <Input
          id="password"
          type="password"
          label={m.password()}
          bind:value={password}
          required
          autocomplete="current-password"
        />
        {#if authConfig.password_auth}
          <div class="text-right">
            <a
              href="/forgot-password"
              class="text-sm text-blue-600 hover:underline dark:text-gray-50"
              >忘记密码？</a
            >
          </div>
        {/if}
        <Button type="submit" {loading} class="w-full"
          >{loading ? m.signin_in() : m.signin()}</Button
        >
      </form>
    {/if}

    {#if configStore.state.demo}
      <div class="text-center">
        <button
          onclick={handleDemo}
          disabled={demoLoading}
          class="text-md text-blue-600 hover:underline disabled:opacity-50 dark:text-gray-50"
        >
          {demoLoading ? m.starting_demo() : m.try_demo()}
        </button>
      </div>
    {/if}
  </div>
</div>
