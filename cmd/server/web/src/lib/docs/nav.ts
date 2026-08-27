export type DocSection = 'help' | 'tech'

export interface DocEntry {
  slug: string
  title: string
  section: DocSection
  order: number
}

export const DOC_NAV: DocEntry[] = [
  // Help
  { section: 'help', slug: 'index', title: 'Overview', order: 0 },
  {
    section: 'help',
    slug: 'getting-started',
    title: 'Getting started',
    order: 1,
  },
  { section: 'help', slug: 'account', title: 'Account settings', order: 1 },
  { section: 'help', slug: 'projects', title: 'Projects & folders', order: 2 },
  { section: 'help', slug: 'assets', title: 'Assets', order: 3 },
  { section: 'help', slug: 'variants', title: 'Variants', order: 4 },
  { section: 'help', slug: 'ai-editing', title: 'AI editing', order: 5 },
  { section: 'help', slug: 'sharing', title: 'Sharing', order: 6 },
  { section: 'help', slug: 'search', title: 'Search & filters', order: 7 },
  { section: 'help', slug: 'exports', title: 'Exporting', order: 8 },
  { section: 'help', slug: 'workflows', title: 'Workflows', order: 9 },
  // Tech
  { section: 'tech', slug: 'index', title: 'Overview', order: 0 },
  { section: 'tech', slug: 'installation', title: 'Installation', order: 1 },
  { section: 'tech', slug: 'configuration', title: 'Configuration', order: 2 },
  { section: 'tech', slug: 'storage', title: 'Storage', order: 3 },
  { section: 'tech', slug: 'upgrade', title: 'Upgrading', order: 4 },
  { section: 'tech', slug: 'oidc', title: 'OIDC / SSO', order: 5 },
  { section: 'tech', slug: 'ingress', title: 'Ingress sources', order: 6 },
  { section: 'tech', slug: 'exports', title: 'Exports', order: 7 },
  { section: 'tech', slug: 'ai-providers', title: 'AI providers', order: 8 },
]

export function navForSection(section: DocSection): DocEntry[] {
  return DOC_NAV.filter((e) => e.section === section).sort(
    (a, b) => a.order - b.order
  )
}

export function findEntry(
  section: DocSection,
  slug: string
): DocEntry | undefined {
  return DOC_NAV.find((e) => e.section === section && e.slug === slug)
}
