<script lang="ts">
  import { ArrowLeft } from '@lucide/svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import TemplateGrid from '$lib/components/workflows/TemplateGrid.svelte'
  import { m } from '$lib/paraglide/messages'
</script>

<svelte:head>
  <title>{m.workflows_new_page_title()} — Damask</title>
</svelte:head>

<div class="flex flex-1 flex-col overflow-hidden">
  <PageHeader
    title={m.workflows_new_page_title()}
    description={m.workflows_new_page_subtitle()}
  >
    <a
      href="/library/settings/workflows"
      class="flex items-center gap-1.5 rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-1.5 text-sm text-[var(--text-secondary)] transition-colors hover:text-[var(--text-primary)]"
    >
      <ArrowLeft class="h-3.5 w-3.5" />
      {m.back()}
    </a>
  </PageHeader>

  <div class="flex grow overflow-y-auto">
    <main class="mx-auto mb-20 flex-1 px-6 py-6">
      <TemplateGrid />
    </main>
  </div>
</div>
