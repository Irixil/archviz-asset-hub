import { extractDescription, loadDoc } from '$lib/docs/loader'
import { findEntry, navForSection } from '$lib/docs/nav'
import { error } from '@sveltejs/kit'
import type { PageLoad } from './$types'

export const prerender = true

export function entries() {
  return navForSection('help')
    .filter((e) => e.slug !== 'index')
    .map((e) => ({ slug: e.slug }))
}

export const load: PageLoad = async ({ params }) => {
  const entry = findEntry('help', params.slug)
  if (!entry) throw error(404, 'Page not found')
  const raw = await loadDoc('help', params.slug)
  const description = extractDescription(raw)
  return { entry, raw, description }
}
