<script lang="ts">
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { authApi, type MeResponse, ApiError } from '$lib/api'
  import LinkedIdentityRow from '$lib/components/LinkedIdentityRow.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import Modal from '$lib/components/ui/Modal.svelte'
  import PageContainer from '$lib/components/ui/PageContainer.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { m } from '$lib/paraglide/messages'
  import { isImage } from '$lib/utils/mime'

  let me = $state<MeResponse | null>(null)
  let loading = $state(true)
  let errors = $state<Record<string, string>>({})
  let savingName = $state(false)
  let displayName = $state('')
  let emailDraft = $state('')
  let requestingEmail = $state(false)
  let emailError = $state('')
  let passwordCurrent = $state('')
  let passwordNext = $state('')
  let passwordConfirm = $state('')
  let passwordSaving = $state(false)
  let passwordError = $state('')
  let avatarUploading = $state(false)
  let avatarInput = $state<HTMLInputElement | null>(null)
  let deleteOpen = $state(false)
  let deletePassword = $state('')
  let deleteLoading = $state(false)
  let deleteError = $state('')

  const hasManualAvatar = $derived(!!me?.avatar_url?.includes('/api/v1/users/'))

  const initials = $derived(
    (me?.display_name || me?.email || '?')
      .split(/\s+/)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase() ?? '')
      .join('')
  )

  async function loadMe() {
    loading = true
    try {
      me = await authApi.me()
      displayName = me.display_name
      emailDraft = me.pending_email ?? ''
    } finally {
      loading = false
    }
  }

  $effect(() => {
    loadMe().catch(() => {})
  })

  $effect(() => {
    if (page.url.searchParams.get('email_changed') === '1') {
      toastStore.show(m.settings_account_email_updated_successfully())
    }
  })

  async function unlink(provider: 'oidc' | 'google' | 'canva') {
    errors = { ...errors, [provider]: '' }
    try {
      if (provider === 'oidc') await authApi.unlinkOIDC()
      else if (provider === 'google') await authApi.unlinkGoogle()
      else await authApi.unlinkCanva()
      await loadMe()
    } catch (err) {
      const msg =
        err instanceof ApiError
          ? err.message
          : m.settings_auth_last_method_error()
      errors = { ...errors, [provider]: msg }
    }
  }

  async function saveDisplayName() {
    if (
      !me ||
      displayName.trim().length === 0 ||
      displayName.trim().length > 100
    )
      return
    savingName = true
    try {
      me = await authApi.updateMe(displayName.trim())
      displayName = me.display_name
      toastStore.show(m.settings_account_display_name_updated())
    } catch (err) {
      toastStore.show(
        err instanceof ApiError
          ? err.message
          : m.settings_account_display_name_update_failed(),
        'error'
      )
    } finally {
      savingName = false
    }
  }

  async function onAvatarChange(event: Event) {
    const input = event.currentTarget as HTMLInputElement
    const file = input.files?.[0]
    if (!file) return
    if (file.size > 5 * 1024 * 1024) {
      toastStore.show(m.settings_account_avatar_file_size_error(), 'error')
      input.value = ''
      return
    }
    if (!isImage(file.type)) {
      toastStore.show(m.settings_account_avatar_file_type_error(), 'error')
      input.value = ''
      return
    }
    avatarUploading = true
    try {
      const uploaded = await authApi.uploadAvatar(file)
      me = { ...uploaded, avatar_url: `${uploaded.avatar_url}?t=${Date.now()}` }
      toastStore.show(m.settings_account_avatar_updated())
    } catch (err) {
      toastStore.show(
        err instanceof ApiError
          ? err.message
          : m.settings_account_avatar_upload_failed(),
        'error'
      )
    } finally {
      avatarUploading = false
      input.value = ''
    }
  }

  async function removeAvatar() {
    try {
      await authApi.deleteAvatar()
      if (me) me = await authApi.me()
      toastStore.show(m.settings_account_avatar_removed())
    } catch (err) {
      toastStore.show(
        err instanceof ApiError
          ? err.message
          : m.settings_account_avatar_remove_failed(),
        'error'
      )
    }
  }

  async function requestEmailChange() {
    if (!emailDraft.trim()) return
    requestingEmail = true
    emailError = ''
    try {
      const result = await authApi.requestEmailChange(emailDraft.trim())
      if (me) me = { ...me, pending_email: result.pending_email }
      toastStore.show(m.settings_account_email_confirmation_sent())
    } catch (err) {
      emailError =
        err instanceof ApiError
          ? err.message
          : m.settings_account_email_change_request_failed()
    } finally {
      requestingEmail = false
    }
  }

  async function cancelEmailChange() {
    try {
      await authApi.cancelPendingEmail()
      if (me) me = { ...me, pending_email: undefined }
      emailDraft = ''
    } catch (err) {
      emailError =
        err instanceof ApiError
          ? err.message
          : m.settings_account_email_change_cancel_failed()
    }
  }

  async function savePassword() {
    if (passwordNext.length < 8) {
      passwordError = m.settings_account_password_too_short()
      return
    }
    if (passwordNext !== passwordConfirm) {
      passwordError = m.settings_account_password_mismatch()
      return
    }
    passwordSaving = true
    passwordError = ''
    try {
      await authApi.changePassword(passwordCurrent, passwordNext)
      passwordCurrent = ''
      passwordNext = ''
      passwordConfirm = ''
      if (me) me = { ...me, has_password: true }
      toastStore.show(m.settings_account_password_updated())
    } catch (err) {
      passwordError =
        err instanceof ApiError
          ? err.message
          : m.settings_account_password_update_failed()
    } finally {
      passwordSaving = false
    }
  }

  async function confirmDelete() {
    deleteLoading = true
    deleteError = ''
    try {
      await authApi.deleteMe(deletePassword)
      goto('/login?account_deleted=1')
    } catch (err) {
      deleteError =
        err instanceof ApiError
          ? err.message
          : m.settings_account_delete_failed()
    } finally {
      deleteLoading = false
    }
  }
</script>

<svelte:head>
  <title>{m.settings_account_title()}｜建筑素材中台</title>
</svelte:head>

<PageContainer>
  <PageHeader title={m.settings_account_title()} />
  <div class="mx-auto w-full max-w-4xl space-y-8 px-8 py-10">
    {#if loading}
      <div class="space-y-8">
        {#each [1, 2, 3] as _}
          <div
            class="h-40 animate-pulse rounded-2xl bg-[var(--bg-elevated)]"
          ></div>
        {/each}
      </div>
    {:else if me}
      <section
        class="rounded-2xl border border-[var(--border)] bg-[var(--bg-surface)] p-6"
      >
        <h2 class="text-base font-semibold text-[var(--text-primary)]">
          {m.settings_account_profile()}
        </h2>
        <div class="mt-5 flex flex-col gap-6 md:flex-row md:items-start">
          <div class="flex items-start gap-4">
            {#if me.avatar_url}
              <img
                src={me.avatar_url}
                alt={me.display_name}
                class="h-20 w-20 shrink-0 rounded-full object-cover ring-1 ring-black/5"
              />
            {:else}
              <div
                class="flex h-20 w-20 shrink-0 items-center justify-center rounded-full text-xl font-semibold"
                style="background: var(--accent-soft); color: var(--accent-text);"
              >
                {initials}
              </div>
            {/if}
            <div class="flex flex-col gap-2 pt-1">
              <input
                bind:this={avatarInput}
                type="file"
                accept="image/*"
                class="hidden"
                onchange={onAvatarChange}
              />
              <Button
                onclick={() => avatarInput?.click()}
                loading={avatarUploading}
              >
                {avatarUploading
                  ? m.settings_account_avatar_uploading()
                  : m.settings_account_avatar_upload()}
              </Button>
              {#if hasManualAvatar}
                <button
                  type="button"
                  class="text-left text-sm text-[var(--text-muted)] transition-colors hover:text-[var(--accent-danger)]"
                  onclick={removeAvatar}
                >
                  {m.settings_account_avatar_remove()}
                </button>
              {/if}
            </div>
          </div>

          <div class="flex-1 space-y-3">
            <Input
              label={m.settings_account_display_name()}
              bind:value={displayName}
            />
            <div class="flex justify-end">
              <Button
                onclick={saveDisplayName}
                disabled={displayName.trim() === me.display_name ||
                  displayName.trim().length === 0}
                loading={savingName}
              >
                {m.settings_account_save()}
              </Button>
            </div>
          </div>
        </div>
      </section>

      <section
        class="rounded-2xl border border-[var(--border)] bg-[var(--bg-surface)] p-6"
      >
        <h2 class="text-base font-semibold text-[var(--text-primary)]">
          {m.settings_account_email_title()}
        </h2>
        <p class="mt-1 text-sm text-[var(--text-muted)]">{me.email}</p>
        {#if me.pending_email}
          <div
            class="mt-4 rounded-lg border border-[var(--border)] bg-[var(--bg-elevated)] px-4 py-3 text-sm text-[var(--text-secondary)]"
          >
            {m.settings_account_email_pending_notice({
              email: me.pending_email,
            })}
            <button
              type="button"
              class="ml-2 text-[var(--text-muted)] underline underline-offset-2 transition-colors hover:text-[var(--accent-danger)]"
              onclick={cancelEmailChange}
            >
              {m.settings_account_cancel()}
            </button>
          </div>
        {:else}
          <div class="mt-4 flex flex-col gap-3 md:flex-row">
            <Input
              class="flex-1"
              label={m.settings_account_new_email()}
              type="email"
              bind:value={emailDraft}
            />
            <div class="md:self-end">
              <Button onclick={requestEmailChange} loading={requestingEmail}>
                {m.settings_account_email_send_confirmation()}
              </Button>
            </div>
          </div>
          <Feedback error={emailError} />
        {/if}
      </section>

      <section
        class="rounded-2xl border border-[var(--border)] bg-[var(--bg-surface)] p-6"
      >
        <h2 class="text-base font-semibold text-[var(--text-primary)]">
          {m.settings_auth_title()}
        </h2>

        <div class="mt-5 grid gap-8 lg:grid-cols-[1.15fr_1fr]">
          <div>
            <h3 class="mb-4 text-sm font-medium text-[var(--text-secondary)]">
              {me.has_password
                ? m.settings_account_change_password()
                : m.settings_account_set_password()}
            </h3>
            <div class="space-y-3">
              {#if me.has_password}
                <Input
                  label={m.settings_account_current_password()}
                  type="password"
                  bind:value={passwordCurrent}
                />
              {/if}
              <Input
                label={m.settings_account_new_password()}
                type="password"
                bind:value={passwordNext}
              />
              <Input
                label={m.settings_account_confirm_new_password()}
                type="password"
                bind:value={passwordConfirm}
              />
              <Feedback error={passwordError} />
              <div class="flex justify-end">
                <Button onclick={savePassword} loading={passwordSaving}>
                  {m.settings_account_save_password()}
                </Button>
              </div>
            </div>
          </div>

          <div
            class="divide-y divide-[var(--border)] rounded-xl border border-[var(--border)] px-5"
          >
            <LinkedIdentityRow
              provider="password"
              label={m.settings_auth_password()}
              linked={me.has_password}
            />
            <LinkedIdentityRow
              provider="google"
              label={m.settings_auth_google()}
              linked={me.google_linked}
              connectHref="/auth/google/login"
              disconnectError={errors.google}
              onDisconnect={() => unlink('google')}
            />
            <LinkedIdentityRow
              provider="oidc"
              label={m.settings_auth_sso()}
              linked={me.oidc_linked}
              connectHref="/auth/oidc/login"
              disconnectError={errors.oidc}
              onDisconnect={() => unlink('oidc')}
            />
            <LinkedIdentityRow
              provider="canva"
              label={m.settings_auth_canva()}
              linked={me.canva_linked}
              connectHref="/auth/canva/login"
              disconnectError={errors.canva}
              onDisconnect={() => unlink('canva')}
            />
          </div>
        </div>
      </section>

      <section
        class="rounded-2xl border border-red-200 bg-red-50/60 p-6 dark:border-red-950/40 dark:bg-red-950/15"
      >
        <h2 class="text-base font-semibold text-red-700 dark:text-red-400">
          {m.settings_account_danger_zone()}
        </h2>
        <p class="mt-2 max-w-xl text-sm text-red-700/75 dark:text-red-300/75">
          {m.settings_account_delete_description()}
        </p>
        <div class="mt-4">
          <Button variant="danger" onclick={() => (deleteOpen = true)}>
            {m.settings_account_delete()}
          </Button>
        </div>
      </section>
    {/if}
  </div>
</PageContainer>

<Modal bind:open={deleteOpen} onclose={() => (deleteOpen = false)}>
  <div class="px-6 py-5">
    <h2 class="text-lg font-semibold text-[var(--text-primary)]">
      {m.settings_account_delete_modal_title()}
    </h2>
    <p class="mt-3 text-sm text-[var(--text-secondary)]">
      {m.settings_account_delete_modal_description()}
    </p>
    {#if me?.has_password}
      <div class="mt-4">
        <Input
          label={m.settings_account_confirm_password()}
          type="password"
          bind:value={deletePassword}
        />
      </div>
    {/if}
    <Feedback error={deleteError} />
    <div class="mt-5 flex justify-end gap-2">
      <Button variant="secondary" onclick={() => (deleteOpen = false)}
        >{m.settings_account_cancel()}</Button
      >
      <Button variant="danger" onclick={confirmDelete} loading={deleteLoading}>
        {m.settings_account_delete_confirm()}
      </Button>
    </div>
  </div>
</Modal>
