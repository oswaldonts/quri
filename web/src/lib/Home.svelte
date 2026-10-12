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
  import { formatMoney, isNegative, isZero, type Currency } from './money'
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
  const live = $derived(shift(thisMonth, -settings.spendOffset))
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

  // Once the offset is known (and whenever it changes) the screen goes back to the live period.
  let offsetSeen = $state<number | null>(null)
  $effect(() => {
    if (!settings.loaded || offsetSeen === settings.spendOffset) return
    offsetSeen = settings.spendOffset
    period = startOf[mode]
  })

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
  <button onclick={() => session.logout()}>Salir</button>
</header>

<main class="page">
  <div class="modes" role="group" aria-label="Vista">
    <button aria-pressed={view === 'movements'} onclick={() => (view = 'movements')}>Movimientos</button>
    <button aria-pressed={view === 'reports'} onclick={() => (view = 'reports')}>Reportes</button>
    <button aria-pressed={view === 'settings'} onclick={() => (view = 'settings')}>Ajustes</button>
  </div>

  {#if view === 'settings'}
    <Settings {period} onchange={() => (currencies.load(), version++)} />
  {:else if settings.loaded}
  <div class="modes" role="group" aria-label="Modo">
    {#each modes as m (m.id)}
      <button aria-pressed={mode === m.id} onclick={() => setMode(m.id)}>{m.label}</button>
    {/each}
  </div>

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
        <section class="currency">
          <h2>{c.currency}</h2>
          <dl class="figures">
            <div>
              <dt>Arrastrado</dt>
              <dd>{formatMoney(c.carried_over, c.currency)}</dd>
              {#if c.carried_from}
                <dd class="detail">
                  {c.carried_from === 'real' ? 'Saldo real' : 'Disponible'} de {shift(period, -1)}
                </dd>
              {/if}
            </div>
            <div>
              <dt>Ingresos</dt>
              <dd>{formatMoney(c.income.total, c.currency)}</dd>
              <dd class="detail">
                Sueldo {formatMoney(c.income.salary, c.currency)} · Extra {formatMoney(c.income.extra, c.currency)}
                {#if !isZero(c.income.exchange)}· Cambios {formatMoney(c.income.exchange, c.currency)}{/if}
              </dd>
            </div>
            <div>
              <dt>Gastado</dt>
              <dd>{formatMoney(c.spent, c.currency)}</dd>
            </div>
            <div>
              <dt>Disponible</dt>
              <dd class:negative={isNegative(c.available)}>{formatMoney(c.available, c.currency)}</dd>
              <dd class="detail">Arrastrado + Ingresos − Gastado</dd>
            </div>
            {#if c.real !== undefined}
              <div>
                <dt>Saldo real</dt>
                <dd>{formatMoney(c.real, c.currency)}</dd>
              </div>
            {/if}
            {#if c.difference !== undefined}
              <div>
                <dt>Diferencia</dt>
                <dd class:negative={isNegative(c.difference)}>{formatMoney(c.difference, c.currency)}</dd>
              </div>
            {/if}
          </dl>

          {#if c.expenses?.length}
            <table>
              <thead>
                <tr>
                  <th>Categoría</th>
                  <th class="num">Gastado</th>
                  <th class="num">Presupuesto</th>
                  <th class="num">Restante</th>
                </tr>
              </thead>
              <tbody>
                {#each c.expenses as e (e.category_id)}
                  <tr>
                    <td>{e.name}</td>
                    <td class="num">{formatMoney(e.spent, c.currency)}</td>
                    <td class="num">{e.budget ? formatMoney(e.budget, c.currency) : '—'}</td>
                    <td class="num" class:negative={e.remaining !== undefined && isNegative(e.remaining)}>
                      {e.remaining ? formatMoney(e.remaining, c.currency) : '—'}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
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
        available={summary?.currencies?.find((x) => x.currency === sheet)?.available ?? '0.00'}
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
