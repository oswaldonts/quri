<script lang="ts">
  import { revealRow } from './flash'
  import DatePicker from './DatePicker.svelte'
  import Select from './Select.svelte'
  import { tick } from 'svelte'
  import { api, connectionError, failureMessage, type Schemas } from './api'
  import { today } from './dates'
  import { compareAmounts, formatMoney, parseAmount, sumAmounts, type Currency } from './money'

  // refresh changes whenever any panel saved something, so every list reloads.
  let { period, currency, planning, refresh, onchange }: {
    period: string
    currency: Currency
    planning: boolean
    refresh: number
    onchange: () => void
  } = $props()

  let all = $state<Schemas['Expense'][]>([])
  const expenses = $derived(all.filter((e) => e.currency === currency))
  let categories = $state<Schemas['Category'][]>([])
  // Exchanges have their own flow: the API refuses a plain expense in a system category.
  const choosable = $derived(categories.filter((c) => !c.system))
  let accounts = $state<Schemas['Account'][]>([])
  const choosableAccounts = $derived(accounts.filter((a) => a.currency === currency))
  let loadError = $state('')

  let open = $state(false)
  let categoryId = $state('')
  let amount = $state('')
  let spentOn = $state(today())
  let description = $state('')
  let tags = $state('')
  let planned = $state(false)
  // A disabled expense stays registered but stops counting (summary and account balance).
  let excluded = $state(false)
  let accountId = $state('')
  let formError = $state('')
  let busy = $state(false)
  let editingId = $state('')
  // Installment this expense is, out of how many; only for expenses that don't repeat yet.
  let occurrences = $state('1')
  let firstNo = $state('1')
  let editingSeries = $state(false)
  let seriesWas = $state({ no: 1, total: 1 })
  const changedSeries = (no: number, total: number) => no !== seriesWas.no || total !== seriesWas.total
  let formEl = $state<HTMLFormElement>()

  $effect(() => {
    void refresh
    const controller = new AbortController()
    const path = { period }
    loadError = ''
    Promise.all([
      api.GET('/periods/{period}/expenses', { params: { path }, signal: controller.signal }),
      api.GET('/periods/{period}/categories', { params: { path }, signal: controller.signal }),
      api.GET('/accounts', { signal: controller.signal }),
    ])
      .then(([exp, cat, acc]) => {
        if (exp.data) all = exp.data
        else loadError = failureMessage(exp.response, exp.error)
        if (acc.data) accounts = acc.data
        if (cat.data) {
          categories = cat.data.categories ?? []
          if (!choosable.some((c) => c.id === categoryId)) categoryId = choosable[0]?.id ?? ''
        }
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

  // The account last used for a currency is the default for the next expense; with none yet,
  // the first bank (usually the one everything is paid from).
  const rememberKey = (c: Currency) => `expense-account:${c}`
  function defaultAccount(): string {
    try {
      const saved = localStorage.getItem(rememberKey(currency))
      if (saved !== null) return choosableAccounts.some((a) => a.id === saved) || saved === '' ? saved : ''
    } catch {
      // storage unavailable: fall through to the first bank
    }
    return (choosableAccounts.find((a) => a.kind === 'bank') ?? choosableAccounts[0])?.id ?? ''
  }
  function remember(id: string) {
    try {
      localStorage.setItem(rememberKey(currency), id)
    } catch {
      // the default just isn't remembered
    }
  }

  // Opening the form for a new expense (or switching currency) preselects the default account.
  $effect(() => {
    void currency
    void choosableAccounts
    if (!editingId) accountId = defaultAccount()
  })

  // The entry just saved: its row is highlighted and scrolled into view.
  let savedId = $state('')
  function markSaved(id: string) {
    savedId = id
    setTimeout(() => savedId === id && (savedId = ''), 3000)
  }
  $effect(() => {
    if (savedId && expenses.some((x) => x.id === savedId)) revealRow(savedId)
  })

  let openCat = $state('')

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    formError = ''
    const parsed = parseAmount(amount)
    if (!parsed) {
      formError = 'Monto inválido: usa números con hasta 2 decimales.'
      return
    }
    const months = Number(occurrences)
    const first = Number(firstNo)
    if (!Number.isInteger(months) || months < 1 || months > 60) {
      formError = 'Total de cuotas: entre 1 y 60.'
      return
    }
    if (!Number.isInteger(first) || first < 1 || first > months) {
      formError = 'La cuota actual va entre 1 y el total de cuotas.'
      return
    }
    busy = true
    try {
      const fields = {
        category_id: categoryId,
        amount: parsed,
        spent_on: spentOn,
        description: description.trim(),
        tags: tags
          .split(',')
          .map((t) => t.trim())
          .filter(Boolean),
        status: planned ? ('planned' as const) : ('confirmed' as const),
        account_id: accountId,
      }
      const { data, error, response } = editingId
        ? await api.PATCH('/periods/{period}/expenses/{id}', {
            params: { path: { period, id: editingId } },
            body: { ...fields, disabled: excluded },
          })
        : await api.POST('/periods/{period}/expenses', {
            params: { path: { period } },
            body: { ...fields, currency, occurrences: months, first_no: first },
          })
      if (error) {
        formError = failureMessage(response, error)
        return
      }
      if (editingId && (editingSeries ? changedSeries(first, months) : months > 1)) {
        const repeated = await api.POST('/periods/{period}/expenses/{id}/repeat', {
          params: { path: { period, id: editingId } },
          body: { occurrences: months, first_no: first },
        })
        if (repeated.error) {
          formError = `Los cambios se guardaron, pero no se pudo repetir: ${failureMessage(repeated.response, repeated.error)}`
          onchange()
          return
        }
      }
      remember(accountId)
      openCat = categoryId
      markSaved(data?.id ?? editingId)
      if (editingId) cancel()
      amount = ''
      description = ''
      tags = ''
      occurrences = '1'
      firstNo = '1'
      onchange()
    } catch {
      formError = connectionError
    } finally {
      busy = false
    }
  }

  async function edit(expense: Schemas['Expense']) {
    editingId = expense.id
    categoryId = expense.category_id
    amount = expense.amount
    spentOn = expense.spent_on
    description = expense.description
    tags = (expense.tags ?? []).join(', ')
    planned = expense.status === 'planned'
    excluded = expense.disabled
    accountId = expense.account_id ?? ''
    editingSeries = !!expense.series_id
    seriesWas = { no: expense.series_no ?? 1, total: expense.series_total ?? 1 }
    firstNo = String(seriesWas.no)
    occurrences = String(seriesWas.total)
    formError = ''
    open = true
    await tick()
    formEl?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }

  function cancel() {
    editingId = ''
    editingSeries = false
    occurrences = '1'
    firstNo = '1'
    open = false
    amount = ''
    description = ''
    tags = ''
    planned = planning
    excluded = false
    accountId = defaultAccount()
    formError = ''
  }

  // A planned expense that happened: it starts counting in the summary.
  async function confirmExpense(expense: Schemas['Expense']) {
    try {
      const { error, response } = await api.PATCH('/periods/{period}/expenses/{id}', {
        params: { path: { period, id: expense.id } },
        body: { status: 'confirmed' },
      })
      if (error) loadError = failureMessage(response, error)
      else onchange()
    } catch {
      loadError = connectionError
    }
  }

  async function remove(expense: Schemas['Expense']) {
    if (!confirm(`¿Eliminar el gasto de ${formatMoney(expense.amount, expense.currency)}?`)) return
    // A recurring expense can go away alone or together with its later installments.
    const following =
      !!expense.series_id &&
      (expense.series_no ?? 0) < (expense.series_total ?? 0) &&
      confirm('Este gasto se repite. Aceptar: eliminar también las cuotas de los meses siguientes. Cancelar: solo esta.')
    try {
      const { error, response } = await api.DELETE('/periods/{period}/expenses/{id}', {
        params: { path: { period, id: expense.id }, query: { following } },
      })
      if (error) loadError = failureMessage(response, error)
      else onchange()
    } catch {
      loadError = connectionError
    }
  }

  const categoryName = (id: string) => categories.find((c) => c.id === id)?.name ?? id
  const accountName = (id: string) => accounts.find((a) => a.id === id)?.name ?? ''

  type CategoryGroup = { id: string; name: string; items: Schemas['Expense'][] }

  // By category, the one with the most spent first (the sums are in the summary above; disabled
  // entries don't count for the order).
  const groups = $derived.by<CategoryGroup[]>(() => {
    const byCategory = new Map<string, Schemas['Expense'][]>()
    for (const e of expenses) byCategory.set(e.category_id, [...(byCategory.get(e.category_id) ?? []), e])
    const counted = (items: Schemas['Expense'][]) => sumAmounts(items.filter((e) => !e.disabled).map((e) => e.amount))
    return [...byCategory]
      .map(([id, items]) => ({ id, name: categoryName(id), items }))
      .sort((a, b) => compareAmounts(counted(b.items), counted(a.items)) || a.name.localeCompare(b.name))
  })

  const grandTotal = $derived(sumAmounts(expenses.filter((e) => !e.disabled).map((e) => e.amount)))
  // Width of a category's bar against everything spent (display only).
  const share = (amount: string, all: string) => {
    const t = Number(all)
    return t > 0 ? Math.round((Number(amount) / t) * 100) : 0
  }
</script>

{#snippet row(e: Schemas['Expense'], fallback: string)}
  <tr id="row-{e.id}" class:off={e.disabled} class:saved={e.id === savedId}>
    <td>{e.spent_on}</td>
    <td>
      {e.description || fallback}
      {#if e.series_id}<span class="badge lime">Cuota {e.series_no}/{e.series_total}</span>{/if}
      {#each e.tags as tag (tag)}<span class="badge">#{tag}</span>{/each}
      {#if e.account_id}<span class="badge">{accountName(e.account_id)}</span>{/if}
      {#if e.status === 'planned'}<span class="badge warn">Planeado</span>{/if}
      {#if e.disabled}<span class="badge">Excluido</span>{/if}
    </td>
    <td class="num"><span class="amount">{formatMoney(e.amount, e.currency)}</span></td>
    <td class="num">
      {#if e.exchange_id}
        {#if e.status === 'planned'}<span class="muted">Se confirma en Cambios de moneda</span>{/if}
      {:else}
        {#if e.status === 'planned'}
          <button class="link neutral" aria-label="Confirmar gasto" onclick={() => confirmExpense(e)}>Confirmar</button>
        {/if}
        <button class="link neutral" aria-label="Editar gasto" onclick={() => edit(e)}>Editar</button>
        <button class="link" aria-label="Eliminar gasto" onclick={() => remove(e)}>Eliminar</button>
      {/if}
    </td>
  </tr>
{/snippet}

<section class="panel" aria-labelledby="expenses-title">
  <div class="panel-head">
    <h2 id="expenses-title">Gastos</h2>
    <button class="primary" aria-expanded={open} onclick={() => (open ? cancel() : (open = true))}>
      {open ? 'Cerrar' : 'Agregar gasto'}
    </button>
  </div>

  {#if open}
    <form class="entry-form" onsubmit={submit} bind:this={formEl}>
      <div class="row wide">
        <label>
          Categoría
          <Select bind:value={categoryId} options={choosable.map((c) => ({ value: c.id, label: c.name }))} />
        </label>
        <label class="grow">
          Descripción
          <input bind:value={description} maxlength="200" placeholder="Chocolate Milky" />
        </label>
        <label>
          Monto en {currency}
          <input bind:value={amount} inputmode="decimal" placeholder="0.00" required />
        </label>
        <label>
          {Number(occurrences) > 1 || editingSeries ? 'Fecha de cobro' : 'Fecha'}
          <DatePicker bind:value={spentOn} />
        </label>
      </div>
      <div class="row wide">
        <label>
          Pagado desde
          <Select
            bind:value={accountId}
            options={[{ value: '', label: 'Sin cuenta' }, ...choosableAccounts.map((a) => ({ value: a.id, label: a.name }))]}
          />
        </label>
        <label>
          Etiquetas (separadas por coma)
          <input bind:value={tags} placeholder="dulces, lácteos" />
        </label>
      </div>
      <div class="row wide">
        <label>
          Cuota actual
          <input type="number" min="1" max={Number(occurrences) || 60} step="1" bind:value={firstNo} />
        </label>
        <label>
          Total de cuotas (1: no se repite)
          <input type="number" min="1" max="60" step="1" bind:value={occurrences} />
        </label>
      </div>
      {#if Number(occurrences) > Number(firstNo)}
        <p class="muted wide">
          {#if editingSeries && changedSeries(Number(firstNo), Number(occurrences))}Las cuotas siguientes se reemplazan por las nuevas. {/if}Cuota {Number(firstNo)} de {Number(occurrences)}: se crean {Number(occurrences) - Number(firstNo)} gastos planeados
          en los periodos siguientes, con el mismo monto y el mismo día de cobro. Confírmalos cuando ocurran.
        </p>
      {/if}
      {#if formError}<p class="error wide" role="alert">{formError}</p>{/if}
      <div class="wide actions">
        <label class="check">
          <input type="checkbox" bind:checked={planned} />
          Planeado (aún no ocurre)
        </label>
        {#if editingId}
          <label class="check" title="Sigue registrado pero no cuenta en el resumen ni en el saldo">
            <input type="checkbox" bind:checked={excluded} />
            Excluir de los totales
          </label>
        {/if}
        <button class="primary" disabled={busy || !amount || !categoryId}>{editingId ? 'Guardar cambios' : 'Guardar gasto'}</button>
        {#if editingId}<button type="button" class="link neutral" onclick={cancel}>Cancelar</button>{/if}
      </div>
    </form>
  {/if}

  {#if loadError}
    <p class="error" role="alert">{loadError}</p>
  {:else if expenses.length}
    <div class="rd-accordion categories">
      {#each groups as c, i (c.id)}
        {@const counted = c.items.filter((e) => !e.disabled)}
        {@const total = sumAmounts(counted.map((e) => e.amount))}
        <details open={i === 0 || c.id === openCat}>
          <summary>
            <span class="cat-name">{c.name} <span class="muted">({c.items.length})</span></span>
            <span class="cat-share" aria-hidden="true">
              <span class="bar-fill" style:width="{share(total, grandTotal)}%"></span>
            </span>
            <span class="cat-total">{formatMoney(total, currency)}</span>
          </summary>
          <div class="rd-accordion__body">
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
                {#each c.items as e (e.id)}
                  {@render row(e, c.name)}
                {/each}
              </tbody>
            </table>
          </div>
        </details>
      {/each}
    </div>
  {:else}
    <p class="muted">Sin gastos en {currency} en este periodo.</p>
  {/if}
</section>
