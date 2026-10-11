import createClient from 'openapi-fetch'
import type { components, paths } from './api-schema'

export type Schemas = components['schemas']

// Same origin as the page: the Go binary serves both in production, and the Vite dev
// server proxies the API paths (see vite.config.ts). The session cookie is HttpOnly, so
// the browser attaches it by itself.
export const api = createClient<paths>({ baseUrl: window.location.origin })

// apiErrorMessage turns an RFC 9457 error body into text for the user: the validation
// details when there are any (they say which field), otherwise the general explanation.
export function apiErrorMessage(error: Schemas['ErrorModel'] | undefined): string {
  const details = error?.errors?.map((e) => e.message).filter(Boolean)
  if (details?.length) return details.join('. ')
  return error?.detail ?? error?.title ?? 'Error inesperado.'
}

export const connectionError = 'No se pudo conectar con el servidor.'

// failureMessage is the text to show for a failed response. It is empty for 401/428:
// the session handles those by swapping the screen (see session.svelte.ts).
export function failureMessage(response: Response, error: Schemas['ErrorModel'] | undefined): string {
  return response.status === 401 || response.status === 428 ? '' : apiErrorMessage(error)
}
