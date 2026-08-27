import { redirect } from '@sveltejs/kit'

export const prerender = false

export function load() {
  redirect(303, '/library')
}
