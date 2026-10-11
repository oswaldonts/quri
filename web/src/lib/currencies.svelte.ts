import { api, type Schemas } from './api'

// The currencies the user registered (Ajustes). Amounts can only be in these, and their
// symbols are what formatMoney shows; a currency without a symbol is shown with its code.
class Currencies {
  list = $state<Schemas['Currency'][]>([])
  loaded = $state(false)

  get codes(): string[] {
    return this.list.map((c) => c.code)
  }

  symbol(code: string): string {
    return this.list.find((c) => c.code === code)?.symbol ?? ''
  }

  async load() {
    try {
      const { data } = await api.GET('/currencies')
      if (data) this.list = data
    } catch {
      // The screens that need the list show their own errors; amounts fall back to the code.
    } finally {
      this.loaded = true
    }
  }
}

export const currencies = new Currencies()
