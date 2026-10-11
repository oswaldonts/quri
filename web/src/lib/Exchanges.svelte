<script lang="ts">
  import { tick } from 'svelte'
  import { api, connectionError, failureMessage, type Schemas } from './api'
  import { today } from './dates'
  import { currencies } from './currencies.svelte'
  import { convert, formatMoney, parseAmount, parseRate, type Currency } from './money'

  // currency is the open sheet: the list shows the exchanges that touch it, and the form
  // starts as "receive this currency".
  let {
    period,
    currency,
    planning,
    refresh,
    onchange,
  }: { period: string; currency: Currency; planning: boolean; refresh: number; onchange: () => void } = $props()

  let all = $state<Schemas['Exchange'][]>([])
  const exchanges = $derived(all.filter((e) => e.from.currency === currency || e.to.currency === currency))
  let loadError = $state('')

  let open = $state(false)
  let from = $state<Currency>('')
  let to = $state<Currency>('')

  // The sheet's currency is what you receive; what you give starts as the first other one registered.
  $effect(() => {
    to = currency
    if (!from || from === currency) from = currencies.codes.find((c) => c !== currency) ?? ''
  })
  let amount = $state('')
  let exchangedOn = $state(today())
  let description = $state('')
  let planned = $state(false)
  let formError = $state('')
  let busy = $state(false)
  // Editing changes amount, date, description and status; the pair and its rate stay as they were.
  let editingId = $state('')
  let formEl = $state<HTMLFormElement>()

  // The period's rate for the pair chosen in the form. undefined: still loading; null: none set.
  let rate = $state<string | null | undefined>(undefined)
  let rateInput = $state('')
  let editingRate = $state(false)
  let rateVersion = $state(0)
  let rateError = $state('')

  const pair = $derived(`${from}-${to}`)
  const samePair = $derived(from === to)
  const noPair = $derived(!from || !to)
  const parsedAmount = $derived(parseAmount(amount))
  const editing = $derived(all.find((e) => e.id === editingId))
  const received = $derived(parsedAmount && rate ? convert(parsedAmount, rate) : null)

  $effect(() => {
    void refresh
    const controller = new AbortController()
    loadError = ''
    api
      .GET('/periods/{period}/exchanges', { params: { path: { period } }, signal: controller.signal })
      .then(({ data, error, response }) => {
        if (data) all = data
        else loadError = failureMessage(response, error)
      })
      .catch(() => {
        if (!controller.signal.aborted) loadError = connectionError
      })
    return () => controller.abort()
  })

  $effect(() => {
    void rateVersion
    if (samePair || noPair) return
    const controller = new AbortController()
    rate = undefined
    editingRate = false
    rateError = ''
    api
      .GET('/periods/{period}/rates/{pair}', { params: { path: { period, pair } }, signal: controller.signal })
      .then(({ data, response }) => {
        if (data) rate = data.rate
        else if (response.status === 404) rate = null
      })
      .catch(() => {
        if (!controller.signal.aborted) rateError = connectionError
      })
    return () => controller.abort()
  })

  $effect(() => {
    planned = planning
  })

  // Not a form of its own (forms can't nest): runs from the button or Enter in the rate field.
  async function saveRate() {
    rateError = ''
    const parsed = parseRate(rateInput)
    if (!parsed) {
      rateError = 'Tasa inválida: un número mayor que cero con hasta 8 decimales.'
      return
    }
    try {
      const { error, response } = await api.PUT('/periods/{period}/rates/{pair}', {
        params: { path: { period, pair } },
        body: { rate: parsed },
      })
      if (error) {
        rateError = failureMessage(response, error)
        return
      }
      rateInput = ''
      rateVersion++
    } catch {
      rateError = connectionError
    }
  }

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    formError = ''
    if (!parsedAmount) {
      formError = 'Monto inválido: usa números con hasta 2 decimales.'
      return
    }
    busy = true
    try {
      const fields = {
        amount: parsedAmount,
        exchanged_on: exchangedOn,
        description: description.trim(),
        status: planned ? ('planned' as const) : ('confirmed' as const),
      }
      const { error, response } = editingId
        ? await api.PATCH('/periods/{period}/exchanges/{id}', {
            params: { path: { period, id: editingId } },
            body: fields,
          })
        : await api.POST('/periods/{period}/exchanges', { params: { path: { period } }, body: { ...fields, from, to } })
      if (error) {
        formError = failureMessage(response, error)
        return
      }
      if (editingId) cancel()
      amount = ''
      description = ''
      onchange()
    } catch {
      formError = connectionError
    } finally {
      busy = false
    }
  }

  async function edit(exchange: Schemas['Exchange']) {
    editingId = exchange.id
    from = exchange.from.currency as Currency
    to = exchange.to.currency as Currency
    amount = exchange.from.amount
    exchangedOn = exchange.exchanged_on
    description = exchange.description
    planned = exchange.status === 'planned'
    formError = ''
    open = true
    await tick()
    formEl?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }

  function cancel() {
    editingId = ''
    open = false
    amount = ''
    description = ''
    planned = planning
    formError = ''
  }

  async function remove(exchange: Schemas['Exchange']) {
    const text = `${formatMoney(exchange.from.amount, exchange.from.currency)} → ${formatMoney(exchange.to.amount, exchange.to.currency)}`
    if (!confirm(`¿Eliminar el cambio ${text}? También se eliminan el gasto y el ingreso que generó.`)) return
    try {
      const { error, response } = await api.DELETE('/periods/{period}/exchanges/{id}', {
        params: { path: { period, id: exchange.id } },
      })
      if (error) loadError = failureMessage(response, error)
      else onchange()
    } catch {
      loadError = connectionError
    }
  }
</script>

<section class="panel" aria-labelledby="exchanges-title">
  <div class="panel-head">
    <h2 id="exchanges-title">Cambios de moneda</h2>
    <button class="primary" aria-expanded={open} onclick={() => (open ? cancel() : (open = true))}>
      {open ? 'Cerrar' : 'Agregar cambio'}
    </button>
  </div>

  {#if open}
    <form class="entry-form" onsubmit={submit} bind:this={formEl}>
      <div class="row wide">
        <label class="grow">
          Descripción
          <input bind:value={description} maxlength="200" placeholder="Casa de cambio" />
        </label>
        <label>
          Das
          <select bind:value={from} disabled={!!editingId}>
            {#each currencies.codes as c (c)}<option value={c}>{c}</option>{/each}
          </select>
        </label>
        <label>
          Recibes
          <select bind:value={to} disabled={!!editingId}>
            {#each currencies.codes as c (c)}<option value={c}>{c}</option>{/each}
          </select>
        </label>
        <label>
          Monto que das
          <input bind:value={amount} inputmode="decimal" placeholder="0.00" required />
        </label>
        <label>
          Fecha
          <input type="date" bind:value={exchangedOn} required />
        </label>
      </div>

      <div class="wide rate-box">
        {#if editing}
          <p class="muted">
            Tasa del cambio: 1 {from} = {editing.rate} {to}. Para cambiar las monedas o la tasa, elimina el cambio y créalo de nuevo.
          </p>
          {#if parsedAmount}
            <p>Recibirás <strong>{formatMoney(convert(parsedAmount, editing.rate), to)}</strong> <span class="muted">(se trunca a centavos)</span></p>
          {/if}
        {:else if noPair}
          <p class="muted">Registra otra moneda en Ajustes para poder hacer cambios.</p>
        {:else if samePair}
          <p class="error">Elige monedas distintas.</p>
        {:else if rate === undefined}
          <p class="muted">Buscando la tasa de {from} a {to}…</p>
        {:else if rate !== null && !editingRate}
          <p>
            Tasa de este periodo: 1 {from} = <strong>{rate}</strong> {to}
            <button type="button" class="link neutral" onclick={() => ((editingRate = true), (rateInput = rate ?? ''))}>
              Cambiar tasa
            </button>
          </p>
          {#if received}
            <p>Recibirás <strong>{formatMoney(received, to)}</strong> <span class="muted">(se trunca a centavos)</span></p>
          {/if}
        {:else}
          <label>
            {rate === null ? `Este periodo no tiene tasa de ${from} a ${to}.` : 'Nueva tasa'} 1 {from} = ? {to}
            <span class="inline">
              <input
                bind:value={rateInput}
                inputmode="decimal"
                placeholder="3.25"
                aria-label="Tasa {from} a {to}"
                onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), saveRate())}
              />
              <button type="button" onclick={saveRate}>Guardar tasa</button>
              {#if rate !== null}
                <button type="button" class="link neutral" onclick={() => (editingRate = false)}>Cancelar</button>
              {/if}
            </span>
          </label>
        {/if}
        {#if rateError}<p class="error" role="alert">{rateError}</p>{/if}
      </div>

      {#if formError}<p class="error wide" role="alert">{formError}</p>{/if}
      <div class="wide actions">
        <label class="check">
          <input type="checkbox" bind:checked={planned} />
          Planeado (aún no ocurre)
        </label>
        <button class="primary" disabled={busy || !amount || (!editingId && (noPair || samePair || !rate))}>{editingId ? 'Guardar cambios' : 'Guardar cambio'}</button>
        {#if editingId}<button type="button" class="link neutral" onclick={cancel}>Cancelar</button>{/if}
      </div>
    </form>
  {/if}

  {#if loadError}
    <p class="error" role="alert">{loadError}</p>
  {:else if exchanges.length}
    <table>
      <thead>
        <tr>
          <th>Fecha</th>
          <th>Detalle</th>
          <th class="num">Das</th>
          <th class="num">Recibes</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each exchanges as e (e.id)}
          <tr>
            <td>{e.exchanged_on}</td>
            <td>
              1 {e.from.currency} = {e.rate} {e.to.currency}{e.description ? ` · ${e.description}` : ''}
              {#if e.status === 'planned'}<span class="badge">Planeado</span>{/if}
            </td>
            <td class="num">{formatMoney(e.from.amount, e.from.currency)}</td>
            <td class="num">{formatMoney(e.to.amount, e.to.currency)}</td>
            <td class="num">
              <button class="link neutral" aria-label="Editar cambio" onclick={() => edit(e)}>Editar</button>
              <button class="link" aria-label="Eliminar cambio" onclick={() => remove(e)}>Eliminar</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {:else}
    <p class="muted">Sin cambios de {currency} en este periodo.</p>
  {/if}
</section>
