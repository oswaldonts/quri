import { api, failureMessage, type Schemas } from './api'

// Preferences of the installation (Ajustes). spendOffset is how many months after being
// earned the money is spent: 0 the same month, 1 the next one.
class AppSettings {
  spendOffset = $state(0)
  defaultCurrency = $state('')
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
    this.spendOffset = data.spend_offset
    this.defaultCurrency = data.default_currency
  }

  // Both return the error message to show, or '' when it was saved.
  async setSpendOffset(offset: number): Promise<string> {
    return this.patch({ spend_offset: offset })
  }

  async setDefaultCurrency(code: string): Promise<string> {
    return this.patch({ default_currency: code })
  }

  private async patch(body: { spend_offset?: number; default_currency?: string }): Promise<string> {
    const { data, error, response } = await api.PATCH('/settings', { body })
    if (data) this.apply(data)
    return error ? failureMessage(response, error) : ''
  }
}

export const settings = new AppSettings()
