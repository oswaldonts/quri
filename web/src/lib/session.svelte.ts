import { api, apiErrorMessage } from './api'

// loading: asking the server; setup: no password yet (first run in local-credentials mode);
// login: password needed; ready: logged in; error: the server could not be reached.
export type SessionState = 'loading' | 'setup' | 'login' | 'ready' | 'error'

class Session {
  state = $state<SessionState>('loading')
  // false in the desktop build, which has no login (and so nothing to log out of).
  authRequired = $state(true)

  // check finds out where the user stands. The API has no "who am I" endpoint, so it
  // asks for something cheap that needs a session: 200 means logged in, 401 means
  // log in, 428 means the password has not been set up yet.
  async check() {
    this.state = 'loading'
    try {
      const health = await api.GET('/health')
      if (health.data) this.authRequired = health.data.auth_required
      const { response } = await api.GET('/categories')
      if (response.ok) this.state = 'ready'
      else if (response.status === 428) this.state = 'setup'
      else if (response.status === 401) this.state = 'login'
      else this.state = 'error'
    } catch {
      this.state = 'error'
    }
  }

  // login returns an error message, or null when it worked.
  async login(password: string): Promise<string | null> {
    const { response, error } = await api.POST('/login', { body: { password } })
    if (response.ok) {
      this.state = 'ready'
      return null
    }
    if (response.status === 401) return 'Contraseña incorrecta.'
    if (response.status === 429) return 'Demasiados intentos. Espera un momento.'
    return apiErrorMessage(error)
  }

  // setup sets the first password and leaves the user logged in.
  async setup(password: string): Promise<string | null> {
    const { response, error } = await api.POST('/setup', { body: { password } })
    if (response.ok) {
      this.state = 'ready'
      return null
    }
    if (response.status === 409) {
      // Another window already did it.
      this.state = 'login'
      return 'La contraseña ya estaba configurada. Inicia sesión.'
    }
    return apiErrorMessage(error)
  }

  async logout() {
    await api.POST('/logout')
    this.state = 'login'
  }
}

export const session = new Session()

// A session can expire (or the server restart with a new secret) while the app is open:
// any response that says so sends the user back to the right screen.
api.use({
  onResponse({ response }) {
    if (session.state !== 'ready') return
    if (response.status === 401) session.state = 'login'
    else if (response.status === 428) session.state = 'setup'
  },
})
