<script lang="ts">
  import { integrationsApi, type OAuthConnection } from '$lib/api'
  import { aiProvidersApi, type ProviderKeyStatus } from '$lib/api/ai_providers'
  import { authStore } from '$lib/stores/auth.svelte'
  import IntegrationCard from '$lib/components/IntegrationCard.svelte'
  import ImageRouterCard from '$lib/components/AIImageRouterCard.svelte'
  import OpenRouterCard from '$lib/components/AIOpenRouterCard.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import { m } from '$lib/paraglide/messages'

  let connections = $state<OAuthConnection[]>([])
  let irStatus = $state<ProviderKeyStatus | null>(null)
  let orStatus = $state<ProviderKeyStatus | null>(null)
  let loading = $state(true)

  $effect(() => {
    Promise.all([integrationsApi.list(), aiProvidersApi.listAIProviders()])
      .then(([c, prs]) => {
        connections = c

        const ir = prs.providers.find((p) => p.id === 'imagerouter')
        irStatus = ir
          ? ({
              key_set: ir?.configured,
              source: ir?.key_source,
            } as unknown as ProviderKeyStatus)
          : null

        const or = prs.providers.find((p) => p.id === 'openrouter')
        orStatus = or
          ? ({
              key_set: or?.configured,
              source: or?.key_source,
            } as unknown as ProviderKeyStatus)
          : null
      })
      .catch(() => {})
      .finally(() => {
        loading = false
      })
  })

  const isOwner = $derived(authStore.role === 'owner')

  function connectionFor(provider: string): OAuthConnection | undefined {
    return connections.find((c) => c.provider === provider)
  }

  async function disconnect(id: string) {
    await integrationsApi.disconnect(id)
    connections = connections.filter((c) => c.id !== id)
  }
</script>

<svelte:head>
  <title>{m.integrations_title()} — Damask</title>
</svelte:head>

<div class="flex-1 overflow-y-auto">
  <PageHeader
    title={m.integrations_title()}
    description={m.integrations_subtitle()}
  />

  <div class="mx-auto w-full max-w-4xl px-8 py-8">
    {#if loading}
      <div class="space-y-3">
        {#each [0, 1] as _}
          <div
            class="flex animate-pulse items-center gap-5 rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-5"
          >
            <div
              class="h-11 w-11 shrink-0 rounded-xl bg-[var(--bg-elevated)]"
            ></div>
            <div class="flex-1 space-y-2">
              <div class="h-3.5 w-32 rounded bg-[var(--bg-elevated)]"></div>
              <div class="h-3 w-56 rounded bg-[var(--bg-elevated)]"></div>
            </div>
            <div
              class="h-8 w-20 shrink-0 rounded-lg bg-[var(--bg-elevated)]"
            ></div>
          </div>
        {/each}
      </div>
    {:else}
      <div class="space-y-6">
        <div class="space-y-3">
          {#if connectionFor('google') !== undefined}
            {@const googleConn = connectionFor('google')!}
            {@const googleNeedsReauth =
              !googleConn.scopes.includes(
                'https://www.googleapis.com/auth/drive.file'
              ) &&
              !googleConn.scopes.includes(
                'https://www.googleapis.com/auth/drive'
              )}
            <IntegrationCard
              provider="google"
              label={m.integrations_google_drive_label()}
              description={m.integrations_google_drive_desc()}
              connected={true}
              providerEmail={googleConn.provider_email}
              connectHref="/integrations/connect/google"
              setupSourceHref={`/settings/ingress/new?connection=${googleConn.id}&type=gdrive`}
              needsReauth={googleNeedsReauth}
              onDisconnect={() => disconnect(googleConn.id)}
            />
          {:else}
            <IntegrationCard
              provider="google"
              label={m.integrations_google_drive_label()}
              description={m.integrations_google_drive_desc()}
              connected={false}
              connectHref="/integrations/connect/google"
            />
          {/if}

          {#if connectionFor('canva') !== undefined}
            {@const canvaConn = connectionFor('canva')!}
            <IntegrationCard
              provider="canva"
              label={m.integrations_canva_label()}
              description={m.integrations_canva_desc()}
              connected={true}
              providerEmail={canvaConn.provider_email}
              connectHref="/integrations/connect/canva"
              setupSourceHref={`/settings/ingress/new?connection=${canvaConn.id}&type=canva`}
              onDisconnect={() => disconnect(canvaConn.id)}
            />
          {:else}
            <IntegrationCard
              provider="canva"
              label={m.integrations_canva_label()}
              description={m.integrations_canva_desc()}
              connected={false}
              connectHref="/integrations/connect/canva"
            />
          {/if}
        </div>

        {#if orStatus !== null || irStatus !== null}
          <div class="space-y-3">
            <h2
              class="text-xs font-semibold tracking-wider text-zinc-400 uppercase dark:text-zinc-500"
            >
              {m.integrations_ai_section_title()}
            </h2>
            <!-- OpenRouter first — preferred provider -->
            {#if orStatus !== null}
              <OpenRouterCard bind:status={orStatus} {isOwner} />
            {/if}
            {#if irStatus !== null}
              <ImageRouterCard bind:status={irStatus} {isOwner} />
            {/if}
          </div>
        {/if}
      </div>
    {/if}
  </div>
</div>
