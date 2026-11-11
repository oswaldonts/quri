<script lang="ts">
  import Select from './Select.svelte'
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
  const viewTitle = { movements: 'Movimientos', reports: 'Reportes', settings: 'Ajustes' }
  const modes: { id: Mode; label: string }[] = [
    { id: 'past', label: 'Pasado' },
    { id: 'current', label: 'Presente' },
    { id: 'plan', label: 'Futuro' },
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
  function toggleTheme() {
    const next = document.documentElement.getAttribute('data-theme') === 'light' ? 'dark' : 'light'
    document.documentElement.classList.add('rd-theme-switching')
    document.documentElement.setAttribute('data-theme', next)
    try {
      localStorage.setItem('rd-theme', next)
    } catch {
      // not persisted
    }
    setTimeout(() => document.documentElement.classList.remove('rd-theme-switching'), 300)
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
  <span class="brand">Income Manager</span>
  <div class="end">
    <button class="icon" aria-label="Cambiar tema" onclick={toggleTheme}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <circle cx="12" cy="12" r="4" />
        <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
      </svg>
    </button>
    <button onclick={() => session.logout()}>Salir</button>
  </div>
</header>

<div class="shell">
  <aside class="side" aria-label="Navegación">
    <nav class="side-group" aria-label="Sección">
      <span class="side-title">Sección</span>
      <div class="side-links">
        {#each ['movements', 'reports'] as const as v (v)}
          <button class="side-link" aria-pressed={view === v} onclick={() => (view = v)}>{viewTitle[v]}</button>
        {/each}
      </div>
    </nav>

    {#if view !== 'settings' && settings.loaded}
      <div class="side-group" role="group" aria-label="Tiempo">
        <span class="side-title">Tiempo</span>
        <div class="side-links">
          {#each modes as m (m.id)}
            <button class="side-link" aria-pressed={mode === m.id} onclick={() => setMode(m.id)}>{m.label}</button>
          {/each}
        </div>
        <div class="period-nav">
          <button aria-label="Mes anterior" disabled={mode === 'current'} onclick={() => go(-1)}>‹</button>
          <input
            type="month"
            bind:value={period}
            disabled={mode === 'current'}
            max={mode === 'past' ? shift(live, -1) : undefined}
            min={mode === 'plan' ? shift(live, 1) : undefined}
            aria-label="Periodo"
          />
          <button aria-label="Mes siguiente" disabled={mode === 'current'} onclick={() => go(1)}>›</button>
        </div>
      </div>

      <div class="side-group" role="group" aria-label="Opciones">
        <span class="side-title">Opciones</span>
        <label class="check">
          <input type="checkbox" checked={projected} disabled={mode === 'plan'} onchange={(e) => (includePlanned = e.currentTarget.checked)} />
          Incluir lo planeado
        </label>
        {#if mode === 'current'}
          <button onclick={closeMonth}>Cerrar mes</button>
          {#if live > calendarLive}
            <button class="link neutral" onclick={reopenMonth}>Reabrir {monthName(shift(live, -1))}</button>
          {/if}
          {#if closeError}<p class="error" role="alert">{closeError}</p>{/if}
        {/if}
      </div>
    {/if}

    <div class="side-foot">
      <button class="side-link" aria-pressed={view === 'settings'} onclick={() => (view = 'settings')}>Ajustes</button>
    </div>
  </aside>

<main class="page">
  <header class="page-head">
    {#if view !== 'settings' && periodPattern.test(period)}
      <span class="eyebrow">{modes.find((m) => m.id === mode)?.label} · {period}</span>
      <h1>{viewTitle[view]}</h1>
    {:else}
      <span class="eyebrow">Configuración</span>
      <h1>{viewTitle[view]}</h1>
    {/if}
  </header>

  {#if view === 'settings'}
    <Settings {period} onchange={() => (currencies.load(), version++)} />
  {:else if settings.loaded}

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
        <Select
          value=""
          label="Agregar moneda"
          placeholder="+ Moneda"
          onchange={(v) => addSheet(String(v))}
          options={currencies.codes.filter((c) => !sheets.includes(c)).map((c) => ({ value: c, label: c }))}
        />
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
              <dl class="mini">
                <div>
                  <dt>Arrastrado</dt>
                  <dd>{money(c.carried_over, c.currency)}</dd>
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
</div>
