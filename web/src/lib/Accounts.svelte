<script lang="ts">
  import Select from './Select.svelte'
  import { api, connectionError, failureMessage, type Schemas } from './api'
  import { formatMoney, isNegative, isZero, parseSigned, subtractAmounts, sumAmounts, type Currency } from './money'

  // available is the Disponible without the cash: the money that should be in the bank accounts.
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
  // The Disponible minus what is in the banks is money not placed in any of them yet; 0 means every
  // sol is placed. The cash is listed but not counted: it is already out of the Disponible.
  const unassigned = $derived(subtractAmounts(available, inBanks))
  let loadError = $state('')

  let adding = $state(false)
  let kind = $state<'bank' | 'cash'>('bank')
  let name = $state('')
  let amount = $state('')
  let editing = $state('')
  // Bank the cash comes from (or goes back to). null: not chosen yet, so the first bank.
  let fromId = $state<string | null>(null)
  const sourceId = $derived(fromId ?? banks[0]?.id ?? '')
  const source = $derived(banks.find((b) => b.id === sourceId))
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

  // Moving money into the cash takes it out of a bank: the bank's balance goes down by the same
  // amount (or up when the cash goes down). Returns an error message, or '' when done.
  async function moveFromBank(delta: string): Promise<string> {
    if (!source || isZero(delta)) return ''
    const { error, response } = await api.PATCH('/accounts/{id}', {
      params: { path: { id: source.id } },
      body: { balance: subtractAmounts(source.balance, delta) },
    })
    return error ? `El efectivo se guardó, pero no se pudo descontar de ${source.name}: ${failureMessage(response, error)}` : ''
  }

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
      if (kind === 'cash') {
        const problem = await moveFromBank(balance)
        if (problem) {
          formError = problem
          onchange()
          return
        }
      }
      adding = false
      name = ''
      amount = ''
      fromId = null
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
    fromId = null
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
      if (account.kind === 'cash') {
        const problem = await moveFromBank(subtractAmounts(balance, account.balance))
        if (problem) {
          formError = problem
          onchange()
          return
        }
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

{#snippet fromBank(delta: string | null)}
  {#if banks.length}
    <label>
      Descontar de
      <Select
        value={sourceId}
        onchange={(v) => (fromId = String(v))}
        options={[...banks.map((b) => ({ value: b.id, label: b.name })), { value: '', label: 'Ninguna' }]}
      />
    </label>
    {#if source && delta && !isZero(delta)}
      <span class="muted">
        {isNegative(delta) ? 'Se devolverán' : 'Se descontarán'}
        {formatMoney(isNegative(delta) ? subtractAmounts('0.00', delta) : delta, currency)}
        {isNegative(delta) ? 'a' : 'de'} {source.name}
      </span>
    {/if}
  {/if}
{/snippet}

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
        <Select
          bind:value={kind}
          options={[
            { value: 'bank', label: 'Banco' },
            { value: 'cash', label: 'Efectivo', disabled: !!cash },
          ]}
        />
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
      {#if kind === 'cash'}{@render fromBank(parseSigned(amount))}{/if}
      {#if formError}<p class="error wide" role="alert">{formError}</p>{/if}
      <div class="wide"><button class="primary" disabled={busy || !amount}>Guardar cuenta</button></div>
    </form>
  {/if}

  {#if loadError}
    <p class="error" role="alert">{loadError}</p>
  {:else if accounts.length}
    <table class="fixed">
      <colgroup><col class="c-date" /><col /><col class="c-amount" /><col class="c-actions" /></colgroup>
      <thead>
        <tr>
          <th colspan="2">Cuenta</th>
          <th class="num">Saldo</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each [...(cash ? [cash] : []), ...banks] as a (a.id)}
          <tr>
            <td colspan="2">
              {a.name}
            </td>
            {#if editing === a.id}
              <td class="num" colspan="2">
                <form class="inline-edit" onsubmit={(e) => save(e, a)}>
                  <input bind:value={editAmount} inputmode="decimal" aria-label="Saldo de {a.name}" required />
                  {#if a.kind === 'cash'}
                    {@const typed = parseSigned(editAmount)}
                    {@render fromBank(typed ? subtractAmounts(typed, a.balance) : null)}
                  {/if}
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
            <td colspan="2">En bancos</td>
            <td class="num">{formatMoney(inBanks, currency)}</td>
            <td></td>
          </tr>
          <tr>
            <td colspan="2">Sin asignar</td>
            <td class="num" class:negative={isNegative(unassigned)}>{formatMoney(unassigned, currency)}</td>
            <td></td>
          </tr>
        </tfoot>
      {/if}
    </table>
    {#if accounts.length && !isZero(unassigned)}
      <p class="muted">
        {isNegative(unassigned)
          ? 'Los bancos suman más que el Disponible.'
          : 'Falta repartir esa plata entre tus bancos: los ingresos no suman solos, escribe el saldo nuevo.'}
      </p>
    {/if}
  {:else}
    <p class="muted">Registra tu efectivo y tus cuentas de banco en {currency} para ver cuánto queda en cada una.</p>
  {/if}
</section>
