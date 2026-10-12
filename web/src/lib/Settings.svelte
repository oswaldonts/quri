<script lang="ts">
  import { onMount } from 'svelte'
  import { api, connectionError, failureMessage } from './api'
  import Categories from './Categories.svelte'
  import { currencies } from './currencies.svelte'
  import { settings } from './settings.svelte'

  let { period, onchange }: { period: string; onchange: () => void } = $props()

  onMount(() => currencies.load())

  type Tab = 'general' | 'currencies' | 'categories'
  const tabs: { id: Tab; label: string }[] = [
    { id: 'general', label: 'General' },
    { id: 'currencies', label: 'Monedas' },
    { id: 'categories', label: 'Categorías' },
  ]
  let tab = $state<Tab>('general')

  let offsetError = $state('')
  let defaultError = $state('')

  async function changeDefault(event: Event) {
    const code = (event.currentTarget as HTMLSelectElement).value
    defaultError = await settings.setDefaultCurrency(code).catch(() => connectionError)
  }

  async function move(index: number, direction: number) {
    const codes = currencies.codes
    const target = index + direction
    if (target < 0 || target >= codes.length) return
    ;[codes[index], codes[target]] = [codes[target], codes[index]]
    error = await currencies.reorder(codes).catch(() => connectionError)
  }

  async function changeOffset(event: Event) {
    const value = Number((event.currentTarget as HTMLSelectElement).value)
    offsetError = await settings.setSpendOffset(value).catch(() => connectionError)
  }

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

<div class="modes" role="group" aria-label="Secciones de Ajustes">
  {#each tabs as t (t.id)}
    <button aria-pressed={tab === t.id} onclick={() => (tab = t.id)}>{t.label}</button>
  {/each}
</div>

{#if tab === 'currencies'}
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
        <th>Orden</th>
        <th>Código</th>
        <th>Símbolo</th>
        <th></th>
      </tr>
    </thead>
    <tbody>
      {#each currencies.list as c, i (c.code)}
        <tr>
          <td>
            <button class="link neutral" aria-label="Subir {c.code}" disabled={i === 0} onclick={() => move(i, -1)}>↑</button>
            <button class="link neutral" aria-label="Bajar {c.code}" disabled={i === currencies.list.length - 1} onclick={() => move(i, 1)}>↓</button>
          </td>
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
    El orden de esta lista es el de las hojas y los selectores. Solo puedes registrar montos en estas monedas. Una moneda se puede quitar mientras no tenga ingresos, gastos, cuentas,
    saldos, presupuestos ni tasas. Si varias comparten símbolo (como el $), déjalo vacío para que se muestre el código.
  </p>
</section>
{:else if tab === 'general'}
<section class="panel" aria-labelledby="default-title">
  <div class="panel-head">
    <h2 id="default-title">Moneda por defecto</h2>
  </div>
  <label>
    Al abrir la app se muestra la hoja de
    <select value={settings.defaultCurrency} onchange={changeDefault} aria-label="Moneda por defecto">
      <option value="">— la primera de la lista —</option>
      {#each currencies.codes as c (c)}<option value={c}>{c}</option>{/each}
    </select>
  </label>
  {#if defaultError}<p class="error" role="alert">{defaultError}</p>{/if}
  <p class="muted">Sin moneda por defecto se abre la última que usaste. El orden de las monedas se cambia en la pestaña Monedas.</p>
</section>

<section class="panel" aria-labelledby="spend-title">
  <div class="panel-head">
    <h2 id="spend-title">Cuándo se gasta lo que ganas</h2>
  </div>
  <label>
    El dinero de un mes se gasta
    <select value={settings.spendOffset} onchange={changeOffset} aria-label="Mes en que se gasta">
      <option value={0}>ese mismo mes</option>
      <option value={1}>el mes siguiente</option>
    </select>
  </label>
  {#if offsetError}<p class="error" role="alert">{offsetError}</p>{/if}
  <p class="muted">
    Si te pagan a fin de mes y gastas ese sueldo el mes siguiente, elige «el mes siguiente». Es una preferencia de
    lectura: no cambia nada de lo registrado, solo en qué mes se dice que se gasta cada periodo. Configúralo al
    empezar; cambiarlo después reinterpreta los periodos ya cargados.
  </p>
</section>

{:else}
<Categories {period} {onchange} />
{/if}
