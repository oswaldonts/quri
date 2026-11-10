<script lang="ts">
  import { onMount } from 'svelte'
  import { api, apiErrorMessage, connectionError, type Schemas } from './api'
  import { monthName, shift, toPeriod } from './dates'
  import Accounts from './Accounts.svelte'
  import Exchanges from './Exchanges.svelte'
  import Expenses from './Expenses.svelte'
  import Incomes from './Incomes.svelte'
  import Reports from './Reports.svelte'
  import Settings from './Settings.svelte'
  import { currencies } from './currencies.svelte'
  import { formatMoney, isNegative, isZero, subtractAmounts, type Currency } from './money'
  import { session } from './session.svelte'
  import { settings } from './settings.svelte'

  const periodPattern = /^\d{4}-\d{2}$/

  // The money earned in a month is spent spendOffset months later (Ajustes), so the period that
  // is "live" is that many months back: with 1, in October the user works with September's money.
  //   past:    periods before the live one (fix what was registered in the wrong place)
  //   current: only the live period (the default)
  //   plan:    this month and later, to plan what comes next
  type Mode = 'past' | 'current' | 'plan'
  const thisMonth = toPeriod(new Date())
  const calendarLive = $derived(shift(thisMonth, -settings.spendOffset))
  // Closing a month moves on to the next one even if the calendar hasn't got there yet.
  const live = $derived(
    settings.closedThrough && shift(settings.closedThrough, 1) > calendarLive ? shift(settings.closedThrough, 1) : calendarLive,
  )
  const modes: { id: Mode; label: string }[] = [
    { id: 'past', label: 'Pasado' },
    { id: 'current', label: 'Actual' },
    { id: 'plan', label: 'Planificar' },
  ]
  const inMode: Record<Mode, (p: string) => boolean> = $derived({
    past: (p) => p < live,
    current: (p) => p === live,
    plan: (p) => p > live,
  })
  const startOf: Record<Mode, string> = $derived({ past: shift(live, -1), current: live, plan: shift(live, 1) })

  let mode = $state<Mode>('current')
  let view = $state<'movements' | 'reports' | 'settings'>('movements')
  // Lets the user see how the planned entries would change what is available.
  let includePlanned = $state(false)
  const projected = $derived(mode === 'plan' || includePlanned)
  let period = $state(toPeriod(new Date()))
  const valid = $derived(periodPattern.test(period) && inMode[mode](period))

  // Once the settings are known (and whenever the live period moves: offset, closing or reopening
  // a month) the screen goes back to the live period.
  let liveSeen = $state('')
  $effect(() => {
    if (!settings.loaded || liveSeen === live) return
    liveSeen = live
    period = startOf[mode]
  })

  let closeError = $state('')
  async function closeMonth() {
    if (!confirm(`¿Cerrar ${monthName(live)}? Pasarás a trabajar con el dinero de ${monthName(shift(live, 1))}. Podrás reabrirlo.`)) return
    closeError = await settings.setClosedThrough(live)
  }
  async function reopenMonth() {
    closeError = await settings.setClosedThrough(shift(live, -2))
  }

  function setMode(next: Mode) {
    mode = next
    period = startOf[next]
  }

  // Moves one month, staying inside the mode's range.
  function go(months: number) {
    const next = shift(period, months)
    if (inMode[mode](next)) period = next
  }

  // One sheet per currency: the whole screen shows a single currency at a time. A sheet exists
  // when the period has movements in it or the user added it (e.g. the currency of a trip).
  let sheet = $state<Currency>('')
  let added = $state<Currency[]>([])
  const sheets = $derived.by<Currency[]>(() => {
    const present = new Set<string>([...added, ...(summary?.currencies ?? []).map((c) => c.currency), sheet])
    return currencies.codes.filter((c) => present.has(c))
  })

  const sheetKey = 'sheet'
  function remembered(): string {
    try {
      return localStorage.getItem(sheetKey) ?? ''
    } catch {
      return ''
    }
  }

  // The first sheet: the default currency (Ajustes), else the one used last time, else the first
  // currency with movements, else the first registered one. A sheet whose currency was removed in Ajustes is replaced the same way.
  $effect(() => {
    if (!currencies.loaded || !settings.loaded) return
    if (sheet && currencies.codes.includes(sheet)) return
    const first = settings.defaultCurrency || remembered()
    if (currencies.codes.includes(first)) sheet = first
    else if (summary || loadError) {
      const used = new Set((summary?.currencies ?? []).map((c) => c.currency))
      sheet = currencies.codes.find((c) => used.has(c)) ?? currencies.codes[0] ?? ''
    }
  })

  $effect(() => {
    if (!sheet) return
    try {
      localStorage.setItem(sheetKey, sheet)
    } catch {
      // Remembering the sheet is a convenience.
    }
  })

  onMount(() => {
    currencies.load()
    settings.load()
  })

  function step(direction: number) {
    sheet = sheets[(sheets.indexOf(sheet) + direction + sheets.length) % sheets.length]
  }

  function addSheet(code: string) {
    if (!currencies.codes.includes(code)) return
    added = [...added, code]
    sheet = code
  }

  let summary = $state<Schemas['Summary'] | null>(null)
  let loadError = $state('')
  let loading = $state(false)
  // Bumped after an income or expense is saved, so the summary is fetched again.
  let version = $state(0)

  // The cash is left out of the Disponible: it isn't controlled as closely as the bank accounts. It is
  // a figure of today, so closed periods keep their plain Disponible.
  let cashBalances = $state<Record<string, string>>({})
  const cashHere = $derived(mode === 'past' ? '0.00' : (cashBalances[sheet] ?? '0.00'))
  const withoutCash = (available: string) => subtractAmounts(available, cashHere)

  // The eye hides every big amount, for when someone looks over your shoulder.
  const hideKey = 'hide-amounts'
  let hideAmounts = $state(false)
  try {
    hideAmounts = localStorage.getItem(hideKey) === '1'
  } catch {
    // storage unavailable: amounts stay visible
  }
  function toggleHide() {
    hideAmounts = !hideAmounts
    try {
      localStorage.setItem(hideKey, hideAmounts ? '1' : '0')
    } catch {
      // not persisted
    }
  }
  const money = (amount: string, currency: Currency) => (hideAmounts ? '••••••' : formatMoney(amount, currency))

  $effect(() => {
    void version
    const controller = new AbortController()
    api
      .GET('/accounts', { signal: controller.signal })
      .then(({ data }) => {
        if (data) cashBalances = Object.fromEntries(data.filter((a) => a.kind === 'cash').map((a) => [a.currency, a.balance]))
      })
      .catch(() => {})
    return () => controller.abort()
  })

  // Reloads whenever the period changes; the cleanup cancels a request that is still in
  // flight, so a slow answer for an old period can't overwrite the current one.
  $effect(() => {
    void version
    void projected
    if (!valid) return
    const controller = new AbortController()
    loading = true
    loadError = ''
    api
      .GET('/periods/{period}/summary', {
        params: { path: { period }, query: { view: projected ? 'projected' : 'confirmed' } },
        signal: controller.signal,
      })
      .then(({ data, error, response }) => {
        if (data) summary = data
        // 401/428 are handled by the session (it swaps this screen for login).
        else if (response.status !== 401 && response.status !== 428) loadError = apiErrorMessage(error)
      })
      .catch(() => {
        if (!controller.signal.aborted) loadError = connectionError
      })
      .finally(() => {
        if (!controller.signal.aborted) loading = false
      })
    return () => controller.abort()
  })
</script>

<header class="bar">
  <h1>Income Manager</h1>
  <div class="modes" role="group" aria-label="Vista">
    <button aria-pressed={view === 'movements'} onclick={() => (view = 'movements')}>Movimientos</button>
    <button aria-pressed={view === 'reports'} onclick={() => (view = 'reports')}>Reportes</button>
    <button aria-pressed={view === 'settings'} onclick={() => (view = 'settings')}>Ajustes</button>
  </div>
  <button onclick={() => session.logout()}>Salir</button>
</header>

<main class="page">

  {#if view === 'settings'}
    <Settings {period} onchange={() => (currencies.load(), version++)} />
  {:else if settings.loaded}
  <div class="modes" role="group" aria-label="Modo">
    {#each modes as m (m.id)}
      <button aria-pressed={mode === m.id} onclick={() => setMode(m.id)}>{m.label}</button>
    {/each}
  </div>

  {#if mode === 'current'}
    <div class="sheets">
      <button onclick={closeMonth}>Cerrar mes</button>
      {#if live > calendarLive}
        <button class="link neutral" onclick={reopenMonth}>Reabrir {monthName(shift(live, -1))}</button>
      {/if}
    </div>
    {#if closeError}<p class="error" role="alert">{closeError}</p>{/if}
  {/if}

  <div class="period">
    {#if mode !== 'current'}
      <button aria-label="Mes anterior" onclick={() => go(-1)}>‹</button>
      <input
        type="month"
        bind:value={period}
        max={mode === 'past' ? shift(live, -1) : undefined}
        min={mode === 'plan' ? shift(live, 1) : undefined}
        aria-label="Periodo"
      />
      <button aria-label="Mes siguiente" onclick={() => go(1)}>›</button>
    {/if}
    {#if periodPattern.test(period)}
      <p class="period-title">
        Dinero de <strong>{monthName(period)}</strong>, se gasta en {monthName(shift(period, settings.spendOffset))}
      </p>
    {/if}
  </div>
  {#if mode !== 'plan'}
    <label class="check">
      <input type="checkbox" bind:checked={includePlanned} />
      Incluir lo planeado (ver cómo cambia el Disponible)
    </label>
  {/if}
  {#if mode === 'past'}
    <p class="muted notice">Estás viendo un periodo pasado: lo que cambies aquí modifica ese mes.</p>
  {:else if mode === 'plan'}
    <p class="muted notice">Modo planificación: los movimientos se marcan como planeados y el resumen los incluye.</p>
  {/if}

  {#if valid}
    <div class="sheets">
      {#if sheets.length > 1}
        <button aria-label="Moneda anterior" onclick={() => step(-1)}>‹</button>
      {/if}
      <div class="segmented" role="group" aria-label="Moneda">
        {#each sheets as code (code)}
          <button aria-pressed={sheet === code} onclick={() => (sheet = code)}>{code}</button>
        {/each}
      </div>
      {#if sheets.length > 1}
        <button aria-label="Moneda siguiente" onclick={() => step(1)}>›</button>
      {/if}
      {#if sheets.length < currencies.codes.length}
        <select aria-label="Agregar moneda" value="" onchange={(e) => (addSheet(e.currentTarget.value), (e.currentTarget.value = ''))}>
          <option value="">+ Moneda</option>
          {#each currencies.codes.filter((c) => !sheets.includes(c)) as c (c)}<option value={c}>{c}</option>{/each}
        </select>
      {/if}
    </div>
  {/if}

  {#if loadError}
    <p class="error" role="alert">{loadError}</p>
  {:else if !valid}
    <p class="muted">Elige un periodo {mode === 'past' ? 'anterior a ' + monthName(live) : 'posterior a ' + monthName(live)}.</p>
  {:else if loading && !summary}
    <p class="muted">Cargando…</p>
  {:else if summary}
    <div class:stale={loading}>
      {#each (summary.currencies ?? []).filter((x) => x.currency === sheet) as c (c.currency)}
        {@const avail = withoutCash(c.available)}
        <section class="currency">
          <div class="hero">
            <div class="tile">
              <div class="label">
                <span>Disponible en {c.currency}</span>
                <button
                  class="icon"
                  aria-label={hideAmounts ? 'Mostrar montos' : 'Ocultar montos'}
                  aria-pressed={hideAmounts}
                  onclick={toggleHide}
                >
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                    <path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12Z" />
                    <circle cx="12" cy="12" r="3" />
                    {#if hideAmounts}<path d="M3 3l18 18" />{/if}
                  </svg>
                </button>
              </div>
              <p class="big" class:negative={isNegative(avail)}>{money(avail, c.currency)}</p>
              {#if isZero(cashHere)}<p class="detail">Arrastrado + Ingresos − Gastado</p>{/if}
              <dl class="mini">
                <div>
                  <dt>Arrastrado</dt>
                  <dd>{money(c.carried_over, c.currency)}</dd>
                  {#if c.carried_from}
                    <dd class="detail">{c.carried_from === 'real' ? 'Saldo real' : 'Disponible'} de {shift(period, -1)}</dd>
                  {/if}
                </div>
                <div>
                  <dt>Gastado</dt>
                  <dd>{money(c.spent, c.currency)}</dd>
                </div>
                {#if c.real !== undefined}
                  <div>
                    <dt>Saldo real</dt>
                    <dd>{money(c.real, c.currency)}</dd>
                  </div>
                {/if}
                {#if c.difference !== undefined}
                  <div>
                    <dt>Diferencia</dt>
                    <dd class:negative={isNegative(c.difference)}>{money(c.difference, c.currency)}</dd>
                  </div>
                {/if}
              </dl>
            </div>
            <div class="tile lime">
              <div class="label"><span>Ingresos</span></div>
              <p class="big">{money(c.income.total, c.currency)}</p>
              <div class="chips">
                <span>Sueldo {money(c.income.salary, c.currency)}</span>
                <span>Extra {money(c.income.extra, c.currency)}</span>
                {#if !isZero(c.income.exchange)}<span>Cambios {money(c.income.exchange, c.currency)}</span>{/if}
              </div>
            </div>
          </div>

          {#if c.expenses?.length}
            <section class="panel" aria-labelledby="categories-summary">
            <h2 id="categories-summary">Gasto por categoría</h2>
            <table class="fixed">
              <colgroup><col class="c-date" /><col /><col class="c-amount" /><col class="c-half" /><col class="c-half" /></colgroup>
              <thead>
                <tr>
                  <th colspan="2">Categoría</th>
                  <th class="num">Gastado</th>
                  <th class="num">Presupuesto</th>
                  <th class="num">Restante</th>
                </tr>
              </thead>
              <tbody>
                {#each c.expenses as e (e.category_id)}
                  <tr>
                    <td colspan="2">
                      {e.name}
                      {#if e.budget && Number(e.budget) > 0}
                        {@const pct = Math.min(100, Math.round((Number(e.spent) / Number(e.budget)) * 100))}
                        <span class="mini-bar" role="img" aria-label="{pct}% del presupuesto">
                          <span class="bar-fill" class:over={Number(e.spent) > Number(e.budget)} style="width: {pct}%"></span>
                        </span>
                      {/if}
                    </td>
                    <td class="num">{money(e.spent, c.currency)}</td>
                    <td class="num">{e.budget ? money(e.budget, c.currency) : '—'}</td>
                    <td class="num" class:negative={e.remaining !== undefined && isNegative(e.remaining)}>
                      {e.remaining ? money(e.remaining, c.currency) : '—'}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
            </section>
          {/if}
        </section>
      {:else}
        <p class="muted">Sin movimientos en {sheet} en este periodo.</p>
      {/each}
    </div>
  {/if}

  {#if valid && view === 'reports'}
    <Reports {period} currency={sheet} {projected} summary={summary?.currencies?.find((x) => x.currency === sheet)} />
  {:else if valid}
    {#if mode === 'current'}
      <Accounts
        currency={sheet}
        available={withoutCash(summary?.currencies?.find((x) => x.currency === sheet)?.available ?? '0.00')}
        refresh={version}
        onchange={() => version++}
      />
    {/if}
    <Incomes {period} currency={sheet} planning={mode === 'plan'} refresh={version} onchange={() => version++} />
    <Exchanges {period} currency={sheet} planning={mode === 'plan'} refresh={version} onchange={() => version++} />
    <Expenses {period} currency={sheet} planning={mode === 'plan'} refresh={version} onchange={() => version++} />
  {/if}
  {/if}
</main>
