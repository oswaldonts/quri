<script lang="ts">
  import { onMount } from 'svelte'
  import { api, connectionError, failureMessage } from './api'
  import Categories from './Categories.svelte'
  import { currencies } from './currencies.svelte'

  let { period, onchange }: { period: string; onchange: () => void } = $props()

  onMount(() => currencies.load())

  let adding = $state(false)
  let code = $state('')
  let symbol = $state('')
  let editing = $state('')
  let editSymbol = $state('')
  let error = $state('')
  let busy = $state(false)

  async function create(event: SubmitEvent) {
    event.preventDefault()
    error = ''
    busy = true
    try {
      const { error: failure, response } = await api.POST('/currencies', {
        body: { code: code.trim(), symbol: symbol.trim() },
      })
      if (failure) {
        error = failureMessage(response, failure)
        return
      }
      adding = false
      code = ''
      symbol = ''
      onchange()
    } catch {
      error = connectionError
    } finally {
      busy = false
    }
  }

  async function save(event: SubmitEvent, current: string) {
    event.preventDefault()
    error = ''
    busy = true
    try {
      const { error: failure, response } = await api.PATCH('/currencies/{code}', {
        params: { path: { code: current } },
        body: { symbol: editSymbol.trim() },
      })
      if (failure) {
        error = failureMessage(response, failure)
        return
      }
      editing = ''
      onchange()
    } catch {
      error = connectionError
    } finally {
      busy = false
    }
  }

  async function remove(current: string) {
    if (!confirm(`¿Quitar la moneda ${current}?`)) return
    error = ''
    try {
      const { error: failure, response } = await api.DELETE('/currencies/{code}', { params: { path: { code: current } } })
      if (failure) error = failureMessage(response, failure)
      else onchange()
    } catch {
      error = connectionError
    }
  }
</script>

<section class="panel" aria-labelledby="currencies-title">
  <div class="panel-head">
    <h2 id="currencies-title">Monedas</h2>
    <button class="primary" aria-expanded={adding} onclick={() => (adding = !adding)}>
      {adding ? 'Cerrar' : 'Agregar moneda'}
    </button>
  </div>

  {#if adding}
    <form class="entry-form" onsubmit={create}>
      <div class="row wide">
        <label>
          Código (3 letras)
          <input bind:value={code} maxlength="3" placeholder="EUR" autocapitalize="characters" required />
        </label>
        <label>
          Símbolo (opcional)
          <input bind:value={symbol} maxlength="4" placeholder="€" />
        </label>
      </div>
      <div class="wide"><button class="primary" disabled={busy || code.trim().length !== 3}>Guardar moneda</button></div>
    </form>
  {/if}

  {#if error}<p class="error" role="alert">{error}</p>{/if}

  <table>
    <thead>
      <tr>
        <th>Código</th>
        <th>Símbolo</th>
        <th></th>
      </tr>
    </thead>
    <tbody>
      {#each currencies.list as c (c.code)}
        <tr>
          <td>{c.code}</td>
          {#if editing === c.code}
            <td colspan="2">
              <form class="inline-edit" onsubmit={(e) => save(e, c.code)}>
                <input bind:value={editSymbol} maxlength="4" aria-label="Símbolo de {c.code}" />
                <button class="primary" disabled={busy}>Guardar</button>
                <button type="button" class="link neutral" onclick={() => (editing = '')}>Cancelar</button>
              </form>
            </td>
          {:else}
            <td>{c.symbol || '—'}</td>
            <td class="num">
              <button class="link neutral" aria-label="Editar {c.code}" onclick={() => ((editing = c.code), (editSymbol = c.symbol))}>Editar</button>
              <button class="link" aria-label="Quitar {c.code}" onclick={() => remove(c.code)}>Quitar</button>
            </td>
          {/if}
        </tr>
      {/each}
    </tbody>
  </table>
  <p class="muted">
    Solo puedes registrar montos en estas monedas. Una moneda se puede quitar mientras no tenga ingresos, gastos, cuentas,
    saldos, presupuestos ni tasas. Si varias comparten símbolo (como el $), déjalo vacío para que se muestre el código.
  </p>
</section>

<Categories {period} {onchange} />
