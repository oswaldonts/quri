<script lang="ts">
  import DatePicker from './DatePicker.svelte'
  import Select from './Select.svelte'
  import { tick } from 'svelte'
  import { api, apiErrorMessage, connectionError, failureMessage, type Schemas } from './api'
  import { today } from './dates'
  import { formatMoney, parseAmount, type Currency } from './money'

  // refresh changes whenever any panel saved something, so every list reloads.
  let { period, currency, planning, refresh, onchange }: {
    period: string
    currency: Currency
    planning: boolean
    refresh: number
    onchange: () => void
  } = $props()

  let all = $state<Schemas['Income'][]>([])
  const incomes = $derived(all.filter((i) => i.currency === currency))
  let categories = $state<Schemas['Category'][]>([])
  const choosable = $derived(categories.filter((c) => !c.system))
  let loadError = $state('')

  let open = $state(false)
  let kind = $state<'salary' | 'extra'>('salary')
  let amount = $state('')
  let receivedOn = $state(today())
  let description = $state('')
  let categoryId = $state('')
  let planned = $state(false)
  let formError = $state('')
  let busy = $state(false)
  // The entry being edited: the form is the same one used to add, filled with its values.
  let editingId = $state('')
  let formEl = $state<HTMLFormElement>()

  // Reload on a new period or after a write; the cleanup drops answers for an old period.
  $effect(() => {
    void refresh
    const controller = new AbortController()
    const path = { period }
    loadError = ''
    Promise.all([
      api.GET('/periods/{period}/incomes', { params: { path }, signal: controller.signal }),
      api.GET('/periods/{period}/categories', { params: { path }, signal: controller.signal }),
    ])
      .then(([inc, cat]) => {
        if (inc.data) all = inc.data
        else loadError = failureMessage(inc.response, inc.error)
        if (cat.data) categories = cat.data.categories ?? []
      })
      .catch(() => {
        if (!controller.signal.aborted) loadError = connectionError
      })
    return () => controller.abort()
  })

  // Entries made while planning are planned by default; the user can still untick it.
  $effect(() => {
    planned = planning
  })

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    formError = ''
    const parsed = parseAmount(amount)
    if (!parsed) {
      formError = 'Monto inválido: usa números con hasta 2 decimales.'
      return
    }
    busy = true
    try {
      const fields = {
        kind,
        amount: parsed,
        received_on: receivedOn,
        description: description.trim(),
        status: planned ? ('planned' as const) : ('confirmed' as const),
      }
      const { error, response } = editingId
        ? await api.PATCH('/periods/{period}/incomes/{id}', {
            params: { path: { period, id: editingId } },
            body: { ...fields, category_id: kind === 'extra' ? categoryId : '' },
          })
        : await api.POST('/periods/{period}/incomes', {
            params: { path: { period } },
            body: { ...fields, currency, ...(kind === 'extra' && categoryId ? { category_id: categoryId } : {}) },
          })
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

  async function edit(income: Schemas['Income']) {
    editingId = income.id
    kind = income.kind === 'extra' ? 'extra' : 'salary'
    amount = income.amount
    receivedOn = income.received_on
    description = income.description
    categoryId = income.category_id ?? ''
    planned = income.status === 'planned'
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

  // A planned income that arrived: it starts counting in the summary.
  async function confirmIncome(income: Schemas['Income']) {
    try {
      const { error, response } = await api.PATCH('/periods/{period}/incomes/{id}', {
        params: { path: { period, id: income.id } },
        body: { status: 'confirmed' },
      })
      if (error) loadError = failureMessage(response, error)
      else onchange()
    } catch {
      loadError = connectionError
    }
  }

  async function remove(income: Schemas['Income']) {
    if (!confirm(`¿Eliminar el ingreso de ${formatMoney(income.amount, income.currency)}?`)) return
    try {
      const { error, response } = await api.DELETE('/periods/{period}/incomes/{id}', {
        params: { path: { period, id: income.id } },
      })
      if (error) loadError = failureMessage(response, error)
      else onchange()
    } catch {
      loadError = connectionError
    }
  }

  const kindLabel = { salary: 'Sueldo', extra: 'Extra', exchange: 'Cambio' }
  const categoryName = (id: string | undefined) => categories.find((c) => c.id === id)?.name ?? id
</script>

<section class="panel" aria-labelledby="incomes-title">
  <div class="panel-head">
    <h2 id="incomes-title">Ingresos</h2>
    <button class="primary" aria-expanded={open} onclick={() => (open ? cancel() : (open = true))}>
      {open ? 'Cerrar' : 'Agregar ingreso'}
    </button>
  </div>

  {#if open}
    <form class="entry-form" onsubmit={submit} bind:this={formEl}>
      <div class="row wide">
        <label>
          Tipo
          <Select
            bind:value={kind}
            options={[
              { value: 'salary', label: 'Sueldo' },
              { value: 'extra', label: 'Extra' },
            ]}
          />
        </label>
        <label class="grow">
          Descripción
          <input bind:value={description} maxlength="200" placeholder="Sueldo julio" />
        </label>
        <label>
          Monto en {currency}
          <input bind:value={amount} inputmode="decimal" placeholder="0.00" required />
        </label>
        <label>
          Fecha
          <DatePicker bind:value={receivedOn} />
        </label>
      </div>
      {#if kind === 'extra'}
        <label class="wide">
          Aporte a una categoría (opcional)
          <Select
            bind:value={categoryId}
            options={[{ value: '', label: '— ninguna —' }, ...choosable.map((c) => ({ value: c.id, label: c.name }))]}
          />
        </label>
      {/if}
      {#if formError}<p class="error wide" role="alert">{formError}</p>{/if}
      <div class="wide actions">
        <label class="check">
          <input type="checkbox" bind:checked={planned} />
          Planeado (aún no ocurre)
        </label>
        <button class="primary" disabled={busy || !amount}>{editingId ? 'Guardar cambios' : 'Guardar ingreso'}</button>
        {#if editingId}<button type="button" class="link neutral" onclick={cancel}>Cancelar</button>{/if}
      </div>
    </form>
  {/if}

  {#if loadError}
    <p class="error" role="alert">{loadError}</p>
  {:else if incomes.length}
    <table class="list">
      <thead>
        <tr>
          <th>Fecha</th>
          <th>Detalle</th>
          <th class="num">Monto</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each incomes as i (i.id)}
          <tr>
            <td>{i.received_on}</td>
            <td>
              {kindLabel[i.kind]}{i.description ? ` · ${i.description}` : ''}
              {#if i.category_id}<span class="muted"> → {categoryName(i.category_id)}</span>{/if}
              {#if i.status === 'planned'}<span class="badge warn">Planeado</span>{/if}
            </td>
            <td class="num">{formatMoney(i.amount, i.currency)}</td>
            <td class="num">
              {#if i.kind === 'exchange'}
                {#if i.status === 'planned'}<span class="muted">Se confirma en Cambios de moneda</span>{/if}
              {:else}
                {#if i.status === 'planned'}
                  <button class="link neutral" aria-label="Confirmar ingreso" onclick={() => confirmIncome(i)}>Confirmar</button>
                {/if}
                <button class="link neutral" aria-label="Editar ingreso" onclick={() => edit(i)}>Editar</button>
                <button class="link" aria-label="Eliminar ingreso" onclick={() => remove(i)}>Eliminar</button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {:else}
    <p class="muted">Sin ingresos en {currency} en este periodo.</p>
  {/if}
</section>
