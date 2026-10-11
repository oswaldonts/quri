import { svelte } from '@sveltejs/vite-plugin-svelte'
import { defineConfig } from 'vite'

// The API lives at the root of the same origin in production (the Go binary serves both),
// so in development Vite forwards these paths to it and the browser still sees one origin:
// the session cookie (SameSite=Strict) and relative fetches work unchanged.
// Add a path here when the API gets a new top-level route.
const apiPaths = [
  '/login',
  '/logout',
  '/setup',
  '/health',
  '/periods',
  '/categories',
  '/accounts',
  '/trends',
  '/openapi.yaml',
  '/docs',
]
const apiURL = process.env.API_URL ?? 'http://127.0.0.1:8080'

// https://vite.dev/config/
export default defineConfig({
  plugins: [svelte()],
  server: {
    proxy: Object.fromEntries(apiPaths.map((path) => [path, apiURL])),
  },
})
