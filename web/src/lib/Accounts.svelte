<script lang="ts">
  import { api, connectionError, failureMessage, type Schemas } from './api'
  import { formatMoney, isNegative, isZero, parseSigned, subtractAmounts, sumAmounts, type Currency } from './money'

  // available is the period's Disponible: the money that should be in the accounts and the cash.
  let { currency, available, refresh, onchange }: {
    currency: Currency
    available: string
    refresh: number
    onchange: () => void
  } = $props()

  let all = $state<Schemas['Account'][]>([])
  const accounts = $derived(all.filter((a) => a.currency === currency))
  const cash = $derived(accounts.find((a) => a.kind === 'cash'))
  const banks = $derived(accounts.filter((a) => a.kind === 'bank'))
  const inBanks = $derived(sumAmounts(banks.map((b) => b.balance)))
  // What is in the accounts: the banks and the cash. The Disponible minus that is money not placed
  // in any of them yet; 0 means every sol is placed.
  const placed = $derived(sumAmounts([inBanks, cash?.balance ?? '0.00']))
  const unassigned = $derived(subtractAmounts(available, placed))
  let loadError = $state('')

  let adding = $state(false)
  let kind = $state<'bank' | 'cash'>('bank')
  let name = $state('')
  let amount = $state('')
  let editing = $state('')
  let editAmount = $state('')
  let formError = $state('')
  let busy = $state(false)

  $effect(() => {
    void refresh
    const controller = new AbortController()
    api
      .GET('/accounts', { signal: controller.signal })
      .then(({ data, error, response }) => {
        if (data) {
          all = data
          loadError = ''
        } else loadError = failureMessage(response, error)
      })
      .catch(() => {
        if (!controller.signal.aborted) loadError = connectionError
      })
    return () => controller.abort()
  })

  // The cash can be added once per currency.
  $effect(() => {
    if (cash && kind === 'cash') kind = 'bank'
  })

  async function create(event: SubmitEvent) {
    event.preventDefault()
    formError = ''
    const balance = parseSigned(amount)
    if (!balance) {
      formError = 'Monto inválido: usa números con hasta 2 decimales.'
      return
    }
    busy = true
    try {
      const { error, response } = await api.POST('/accounts', {
        body: { kind, currency, name: kind === 'cash' ? '' : name.trim(), balance },
      })
      if (error) {
        formError = failureMessage(response, error)
        return
      }
      adding = false
      name = ''
      amount = ''
      onchange()
    } catch {
      formError = connectionError
    } finally {
      busy = false
    }
  }

  function edit(account: Schemas['Account']) {
    editing = account.id
    editAmount = account.balance
    formError = ''
  }

  async function save(event: SubmitEvent, account: Schemas['Account']) {
    event.preventDefault()
    formError = ''
    const balance = parseSigned(editAmount)
    if (!balance) {
      formError = 'Monto inválido: usa números con hasta 2 decimales.'
      return
    }
    busy = true
    try {
      const { error, response } = await api.PATCH('/accounts/{id}', {
        params: { path: { id: account.id } },
        body: { balance },
      })
      if (error) {
        formError = failureMessage(response, error)
        return
      }
      editing = ''
      onchange()
    } catch {
      formError = connectionError
    } finally {
      busy = false
    }
  }

  async function remove(account: Schemas['Account']) {
    if (!confirm(`¿Eliminar la cuenta ${account.name}?`)) return
    try {
      const { error, response } = await api.DELETE('/accounts/{id}', { params: { path: { id: account.id } } })
      if (error) loadError = failureMessage(response, error)
      else onchange()
    } catch {
      loadError = connectionError
    }
  }
</script>

<section class="panel" aria-labelledby="accounts-title">
  <div class="panel-head">
    <h2 id="accounts-title">Cuentas y efectivo</h2>
    <button class="primary" aria-expanded={adding} onclick={() => (adding = !adding)}>
      {adding ? 'Cerrar' : 'Agregar cuenta'}
    </button>
  </div>

  {#if adding}
    <form class="entry-form" onsubmit={create}>
      <label>
        Tipo
        <select bind:value={kind}>
          <option value="bank">Banco</option>
          <option value="cash" disabled={!!cash}>Efectivo</option>
        </select>
      </label>
      {#if kind === 'bank'}
        <label>
          Nombre
          <input bind:value={name} maxlength="40" placeholder="BCP" required />
        </label>
      {/if}
      <label>
        Saldo en {currency}
        <input bind:value={amount} inputmode="decimal" placeholder="0.00" required />
      </label>
      {#if formError}<p class="error wide" role="alert">{formError}</p>{/if}
      <div class="wide"><button class="primary" disabled={busy || !amount}>Guardar cuenta</button></div>
    </form>
  {/if}

  {#if loadError}
    <p class="error" role="alert">{loadError}</p>
  {:else if accounts.length}
    <table>
      <thead>
        <tr>
          <th>Cuenta</th>
          <th class="num">Saldo</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each [...(cash ? [cash] : []), ...banks] as a (a.id)}
          <tr>
            <td>
              {a.name}
              {#if a.balance !== a.amount}
                <span class="muted">(registraste {formatMoney(a.amount, a.currency)})</span>
              {/if}
            </td>
            {#if editing === a.id}
              <td class="num" colspan="2">
                <form class="inline-edit" onsubmit={(e) => save(e, a)}>
                  <input bind:value={editAmount} inputmode="decimal" aria-label="Saldo de {a.name}" required />
                  <button class="primary" disabled={busy}>Guardar</button>
                  <button type="button" class="link neutral" onclick={() => (editing = '')}>Cancelar</button>
                </form>
                {#if formError}<span class="error" role="alert">{formError}</span>{/if}
              </td>
            {:else}
              <td class="num" class:negative={isNegative(a.balance)}>{formatMoney(a.balance, a.currency)}</td>
              <td class="num">
                <button class="link neutral" aria-label="Editar {a.name}" onclick={() => edit(a)}>Editar</button>
                <button class="link" aria-label="Eliminar {a.name}" onclick={() => remove(a)}>Eliminar</button>
              </td>
            {/if}
          </tr>
        {/each}
      </tbody>
      {#if accounts.length}
        <tfoot>
          <tr>
            <td>En cuentas{cash ? ' y efectivo' : ''}</td>
            <td class="num">{formatMoney(placed, currency)}</td>
            <td></td>
          </tr>
          <tr>
            <td>Sin asignar <span class="muted">(Disponible − cuentas)</span></td>
            <td class="num" class:negative={isNegative(unassigned)}>{formatMoney(unassigned, currency)}</td>
            <td></td>
          </tr>
        </tfoot>
      {/if}
    </table>
    {#if accounts.length && !isZero(unassigned)}
      <p class="muted">
        {isNegative(unassigned)
          ? 'Las cuentas suman más que el Disponible.'
          : 'Falta repartir esa plata entre tus cuentas: los ingresos no suman solos, escribe el saldo nuevo.'}
      </p>
    {/if}
    <p class="muted">Cada gasto descuenta de la cuenta que elijas al registrarlo. El efectivo es una cuenta más: sus gastos bajan el Disponible como los demás.</p>
  {:else}
    <p class="muted">Registra tu efectivo y tus cuentas de banco en {currency} para ver cuánto queda en cada una.</p>
  {/if}
</section>
