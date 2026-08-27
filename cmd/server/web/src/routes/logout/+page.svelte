<script lang="ts">
  import { onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import { authStore } from '$lib/stores/auth.svelte'
  import { apiFetch } from '$lib/api'
  import { m } from '$lib/paraglide/messages'

  onMount(async () => {
    await apiFetch('/auth/logout', { method: 'POST' }).catch(() => {})
    authStore.logout()
    goto('/login')
  })
</script>

<p>{m.signin_out()}</p>
