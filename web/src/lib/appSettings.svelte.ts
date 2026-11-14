import { api, failureMessage, type Schemas } from './api'

// Preferences of the installation (Ajustes).
class AppSettings {
  defaultCurrency = $state('')
  // Last period the user closed ('' none): the work moves on to the next one before the calendar does.
  closedThrough = $state('')
  loaded = $state(false)

  async load() {
    try {
      const { data } = await api.GET('/settings')
      if (data) this.apply(data)
      this.loaded = !!data
    } catch {
      // Home keeps waiting; the rest of the screens show their own errors.
    }
  }

  private apply(data: Schemas['Settings']) {
    this.defaultCurrency = data.default_currency
    this.closedThrough = data.closed_through
  }

  // Each returns the error message to show, or '' when it was saved.
  async setDefaultCurrency(code: string): Promise<string> {
    return this.patch({ default_currency: code })
  }

  async setClosedThrough(period: string): Promise<string> {
    return this.patch({ closed_through: period })
  }

  private async patch(body: { default_currency?: string; closed_through?: string }): Promise<string> {
    const { data, error, response } = await api.PATCH('/settings', { body })
    if (data) this.apply(data)
    return error ? failureMessage(response, error) : ''
  }
}

export const settings = new AppSettings()
