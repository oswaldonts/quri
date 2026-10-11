<script lang="ts">
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
  const inCurrency = $derived(all.filter((e) => e.currency === currency))
  // Expenses paid from the cash are listed apart: they don't count in the summary, only lower the cash.
  const expenses = $derived(inCurrency.filter((e) => !e.cash))
  const cashExpenses = $derived(inCurrency.filter((e) => e.cash))
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
  let accountId = $state('')
  let formError = $state('')
  let busy = $state(false)
  let editingId = $state('')
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
      const { error, response } = editingId
        ? await api.PATCH('/periods/{period}/expenses/{id}', {
            params: { path: { period, id: editingId } },
            body: fields,
          })
        : await api.POST('/periods/{period}/expenses', { params: { path: { period } }, body: { ...fields, currency } })
      if (error) {
        formError = failureMessage(response, error)
        return
      }
      remember(accountId)
      if (editingId) cancel()
      amount = ''
      description = ''
      tags = ''
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
    accountId = expense.account_id ?? ''
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
    tags = ''
    planned = planning
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
    try {
      const { error, response } = await api.DELETE('/periods/{period}/expenses/{id}', {
        params: { path: { period, id: expense.id } },
      })
      if (error) loadError = failureMessage(response, error)
      else onchange()
    } catch {
      loadError = connectionError
    }
  }

  const categoryName = (id: string) => categories.find((c) => c.id === id)?.name ?? id
  const accountName = (id: string) => accounts.find((a) => a.id === id)?.name ?? ''

  type CategoryGroup = { id: string; name: string; total: string; planned: string; items: Schemas['Expense'][] }

  // Totals count what happened; planned entries are shown apart so they don't inflate them.
  const happened = (items: Schemas['Expense'][]) => sumAmounts(items.filter((e) => e.status !== 'planned').map((e) => e.amount))
  const upcoming = (items: Schemas['Expense'][]) => sumAmounts(items.filter((e) => e.status === 'planned').map((e) => e.amount))

  // By category, biggest first.
  const groups = $derived.by<CategoryGroup[]>(() => {
    const byCategory = new Map<string, Schemas['Expense'][]>()
    for (const e of expenses) byCategory.set(e.category_id, [...(byCategory.get(e.category_id) ?? []), e])
    return [...byCategory]
      .map(([id, items]) => ({ id, name: categoryName(id), total: happened(items), planned: upcoming(items), items }))
      .sort((a, b) => compareAmounts(b.total, a.total) || a.name.localeCompare(b.name))
  })
</script>


{#snippet row(e: Schemas['Expense'], fallback: string)}
  <tr>
    <td>{e.spent_on}</td>
    <td>
      {e.description || fallback}
      {#each e.tags as tag (tag)}<span class="badge">#{tag}</span>{/each}
      {#if e.account_id}<span class="badge">{accountName(e.account_id)}</span>{/if}
      {#if e.status === 'planned'}<span class="badge">Planeado</span>{/if}
    </td>
    <td class="num">{formatMoney(e.amount, e.currency)}</td>
    <td class="num">
      {#if !e.exchange_id}
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
          <select bind:value={categoryId} required>
            {#each choosable as c (c.id)}<option value={c.id}>{c.name}</option>{/each}
          </select>
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
          Fecha
          <input type="date" bind:value={spentOn} required />
        </label>
      </div>
      <div class="row wide">
        <label>
          Pagado desde
          <select bind:value={accountId}>
            <option value="">Sin cuenta</option>
            {#each choosableAccounts as a (a.id)}<option value={a.id}>{a.name}</option>{/each}
          </select>
        </label>
        <label>
          Etiquetas (separadas por coma)
          <input bind:value={tags} placeholder="dulces, lácteos" />
        </label>
      </div>
      {#if accountId && choosableAccounts.find((a) => a.id === accountId)?.kind === 'cash'}
        <p class="muted wide">El efectivo no forma parte del Disponible: este gasto solo baja el efectivo.</p>
      {/if}
      {#if formError}<p class="error wide" role="alert">{formError}</p>{/if}
      <div class="wide actions">
        <label class="check">
          <input type="checkbox" bind:checked={planned} />
          Planeado (aún no ocurre)
        </label>
        <button class="primary" disabled={busy || !amount || !categoryId}>{editingId ? 'Guardar cambios' : 'Guardar gasto'}</button>
        {#if editingId}<button type="button" class="link neutral" onclick={cancel}>Cancelar</button>{/if}
      </div>
    </form>
  {/if}

  {#if loadError}
    <p class="error" role="alert">{loadError}</p>
  {:else if inCurrency.length}
    {#each groups as c (c.id)}
      <details class="group">
        <summary>
          <span class="group-title">{c.name} <span class="muted">({c.items.length})</span></span>
          <span class="group-total">
            {formatMoney(c.total, currency)}
            {#if compareAmounts(c.planned, '0') > 0}
              <span class="muted">+ {formatMoney(c.planned, currency)} planeado</span>
            {/if}
          </span>
        </summary>
        <table>
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
      </details>
    {/each}
    {#if cashExpenses.length}
      <details class="group">
        <summary>
          <span class="group-title">Efectivo <span class="muted">({cashExpenses.length})</span></span>
          <span class="group-total">{formatMoney(happened(cashExpenses), currency)}</span>
        </summary>
        <table>
          <thead>
            <tr>
              <th>Fecha</th>
              <th>Detalle</th>
              <th class="num">Monto</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each cashExpenses as e (e.id)}
              {@render row(e, categoryName(e.category_id))}
            {/each}
          </tbody>
        </table>
      </details>
    {/if}
  {:else}
    <p class="muted">Sin gastos en {currency} en este periodo.</p>
  {/if}
</section>
