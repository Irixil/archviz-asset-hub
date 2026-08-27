<script lang="ts">
  import { onMount } from 'svelte'
  import { formatDistanceToNowStrict } from 'date-fns'
  import { zhCN } from 'date-fns/locale'
  import { UserPlus } from '@lucide/svelte'
  import { workspaceApi } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import type { WorkspaceMember, WorkspaceInvite } from '$lib/api'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Badge from '$lib/components/ui/Badge.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import SectionHeading from '$lib/components/ui/SectionHeading.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'
  import ButtonDelete from '$lib/components/ui/ButtonDelete.svelte'
  import ButtonCancel from '$lib/components/ui/ButtonCancel.svelte'
  import { m } from '$lib/paraglide/messages'

  const currentUserID = $derived(authStore.user?.id)
  const isOwner = $derived(authStore.role === 'owner')

  let members = $state<WorkspaceMember[]>([])
  let invites = $state<WorkspaceInvite[]>([])
  let loading = $state(true)
  let confirmRemoveID = $state<string | null>(null)

  let showInviteForm = $state(false)
  let inviteEmail = $state('')
  let inviteRole = $state<'editor' | 'viewer'>('editor')
  let inviting = $state(false)

  let updatingRole = $state<string | null>(null)
  let _removingID = $state<string | null>(null)
  let deletingInviteID = $state<string | null>(null)

  async function load() {
    loading = true
    try {
      ;[members, invites] = await Promise.all([
        workspaceApi.listMembers(),
        workspaceApi.listInvites(),
      ])
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.members_load_failed(),
        'error'
      )
    } finally {
      loading = false
    }
  }

  onMount(load)

  async function changeRole(userId: string, role: string) {
    updatingRole = userId
    try {
      await workspaceApi.updateMemberRole(userId, role)
      members = members.map((mb) =>
        mb.user_id === userId ? { ...mb, role } : mb
      )
      toastStore.show('成员角色已更新')
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.role_update_failed(),
        'error'
      )
    } finally {
      updatingRole = null
    }
  }

  async function removeMember(userId: string) {
    _removingID = userId
    try {
      await workspaceApi.removeMember(userId)
      members = members.filter((mb) => mb.user_id !== userId)
      toastStore.show('成员已移除')
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.member_remove_failed(),
        'error'
      )
    } finally {
      _removingID = null
      confirmRemoveID = null
    }
  }

  async function cancelInvite(inviteId: string) {
    deletingInviteID = inviteId
    try {
      await workspaceApi.deleteInvite(inviteId)
      invites = invites.filter((i) => i.id !== inviteId)
      toastStore.show('邀请已取消')
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.invite_cancel_failed(),
        'error'
      )
    } finally {
      deletingInviteID = null
    }
  }

  async function sendInvite() {
    if (!inviteEmail.trim()) return
    inviting = true
    try {
      await workspaceApi.createInvite(inviteEmail.trim(), inviteRole)
      toastStore.show(`邀请已发送至 ${inviteEmail.trim()}`)
      inviteEmail = ''
      inviteRole = 'editor'
      showInviteForm = false
      invites = await workspaceApi.listInvites()
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.invite_sent_failed(),
        'error'
      )
    } finally {
      inviting = false
    }
  }
</script>

<svelte:head>
  <title>{m.tab_members()}｜建筑素材中台</title>
</svelte:head>

<div class="flex-1 overflow-y-auto">
  <PageHeader title={m.tab_members()} />
  <div class="mx-auto w-full max-w-3xl space-y-8 px-8 py-10">
    {#if !isOwner}
      <EmptyState
        title={m.owner_access_required()}
        description={m.members_settings_owner_only()}
      />
    {:else if loading}
      <div class="flex justify-center py-12">
        <Spinner />
      </div>
    {:else}
      <!-- Members -->
      <section>
        <div class="flex-1">
          <div class="mb-3 flex items-center justify-between">
            <SectionHeading
              titleClass="text-xl"
              title={m.members()}
              count={members.length}
            />
            <Button
              variant="primary"
              onclick={() => {
                showInviteForm = !showInviteForm
              }}
            >
              <UserPlus class="mr-1.5 h-3.5 w-3.5" />
              {m.member_invite()}
            </Button>
          </div>
          <Hint>{m.member_invite_description()}</Hint>
        </div>
      </section>

      <section>
        {#if showInviteForm}
          <div
            class="mb-4 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-elevated)] p-4"
          >
            <div class="flex gap-2">
              <Input
                type="email"
                placeholder={m.email()}
                bind:value={inviteEmail}
                class="flex-1"
              />
              <select
                bind:value={inviteRole}
                class="rounded-md border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-1.5 text-sm text-[var(--text-primary)]
                       focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 focus:outline-none"
              >
                <option value="editor">{m.editor()}</option>
                <option value="viewer">{m.viewer()}</option>
              </select>
              <Button
                onclick={sendInvite}
                loading={inviting}
                disabled={!inviteEmail.trim()}
              >
                {m.send()}
              </Button>
              <ButtonCancel
                x
                onclick={() => {
                  showInviteForm = false
                }}
              />
            </div>
          </div>
        {/if}

        <div
          class="divide-y divide-[var(--border-subtle)] rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)]"
        >
          {#each members as member (member.user_id)}
            <div class="flex items-center gap-3 px-4 py-3">
              <!-- Avatar -->
              <div
                class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-indigo-100 text-xs font-semibold text-indigo-700 dark:bg-indigo-900/50 dark:text-indigo-300"
              >
                {member.name.slice(0, 2).toUpperCase()}
              </div>

              <!-- Info -->
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span
                    class="truncate text-sm font-medium text-[var(--text-primary)]"
                    >{member.name}</span
                  >
                  {#if member.user_id === currentUserID}
                    <span class="text-xs text-[var(--text-muted)]"
                      >（当前账号）</span
                    >
                  {/if}
                </div>
                <div class="truncate text-xs text-[var(--text-muted)]">
                  {member.email}
                </div>
              </div>

              <!-- Role -->
              {#if member.user_id === currentUserID || member.role === 'owner'}
                <Badge variant={member.role as 'owner' | 'editor' | 'viewer'}
                  >{member.role === 'owner'
                    ? m.owner()
                    : member.role === 'editor'
                      ? m.editor()
                      : m.viewer()}</Badge
                >
              {:else if updatingRole === member.user_id}
                <Spinner size="sm" />
              {:else}
                <select
                  value={member.role}
                  onchange={(e) =>
                    changeRole(
                      member.user_id,
                      (e.target as HTMLSelectElement).value
                    )}
                  class="rounded border border-[var(--border)] bg-[var(--bg-surface)] py-0.5 pr-6 pl-2 text-xs text-[var(--text-primary)]
                         focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 focus:outline-none"
                >
                  <option value="owner">{m.owner()}</option>
                  <option value="editor">{m.editor()}</option>
                  <option value="viewer">{m.viewer()}</option>
                </select>
              {/if}

              <!-- Remove -->
              {#if member.user_id !== currentUserID}
                {#if confirmRemoveID === member.user_id}
                  <div class="flex items-center gap-1.5">
                    <span class="text-xs text-[var(--text-muted)]"
                      >{m.remove()}?</span
                    >
                    <ButtonDelete
                      title="移除成员"
                      onclick={() => removeMember(member.user_id)}
                      >{m.yes()}</ButtonDelete
                    >
                    <ButtonCancel
                      onclick={() => {
                        confirmRemoveID = null
                      }}
                    />
                  </div>
                {:else}
                  <ButtonDelete
                    title="移除成员"
                    onclick={() => {
                      confirmRemoveID = member.user_id
                    }}
                  />
                {/if}
              {/if}
            </div>
          {/each}

          {#if members.length === 0}
            <div class="px-4 py-8 text-center text-sm text-[var(--text-muted)]">
              {m.no_members_yet()}
            </div>
          {/if}
        </div>
      </section>

      <!-- Pending invites -->
      {#if invites.length > 0}
        <section>
          <SectionHeading
            titleClass="text-xl"
            title={m.pending_invites()}
            count={invites.length}
          />
          <Hint>{m.pending_invites_description()}</Hint>
        </section>
        <section>
          <div
            class="divide-y divide-[var(--border-subtle)] rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)]"
          >
            {#each invites as invite (invite.id)}
              <div class="flex items-center gap-3 px-4 py-3">
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm text-[var(--text-primary)]">
                    {invite.email}
                  </div>
                  <div class="text-xs text-[var(--text-muted)]">
                    将于 {formatDistanceToNowStrict(
                      new Date(invite.expires_at),
                      { addSuffix: true, locale: zhCN }
                    )}过期
                  </div>
                </div>
                <Badge variant={invite.role as 'editor' | 'viewer'}
                  >{invite.role}</Badge
                >
                <ButtonDelete
                  x
                  title={m.cancel()}
                  onclick={() => cancelInvite(invite.id)}
                  loading={deletingInviteID === invite.id}
                />
              </div>
            {/each}
          </div>
        </section>
      {/if}
    {/if}
  </div>
</div>
