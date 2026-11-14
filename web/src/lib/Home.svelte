<script lang="ts">
  import Logo from './Logo.svelte'
  import Select from './Select.svelte'
  import MonthPicker from './MonthPicker.svelte'
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
  import { settings } from './appSettings.svelte'

  const periodPattern = /^\d{4}-\d{2}$/

  // The live period is the calendar month, or the one after the last closed month when that is later.
  // The mode follows the chosen month:
  //   past:    before the live one (fix what was registered in the wrong place)
  //   current: the live one (the default)
  //   plan:    after it, to plan what comes next
  type Mode = 'past' | 'current' | 'plan'
  const thisMonth = toPeriod(new Date())
  // Closing a month moves on to the next one even if the calendar hasn't got there yet.
  const live = $derived(
    settings.closedThrough && shift(settings.closedThrough, 1) > thisMonth ? shift(settings.closedThrough, 1) : thisMonth,
  )
  const viewTitle = { movements: 'Movimientos', reports: 'Reportes', settings: 'Ajustes' }
  const modeLabel: Record<Mode, string> = { past: 'Pasado', current: 'Presente', plan: 'Futuro' }

  let collapsed = $state(false)
  let drawer = $state(false)
  try {
    collapsed = localStorage.getItem('rd-sidebar-app') === '1'
  } catch {
    // not persisted
  }
  function toggleCollapsed() {
    collapsed = !collapsed
    try {
      localStorage.setItem('rd-sidebar-app', collapsed ? '1' : '0')
    } catch {
      // not persisted
    }
  }

  let view = $state<'movements' | 'reports' | 'settings'>('movements')
  // Lets the user see how the planned entries would change what is available.
  let includePlanned = $state(false)
  let period = $state(toPeriod(new Date()))
  const valid = $derived(periodPattern.test(period))
  const mode = $derived<Mode>(period < live ? 'past' : period === live ? 'current' : 'plan')
  const projected = $derived(mode === 'plan' || includePlanned)

  // Once the settings are known (and whenever the live period moves: offset, closing or reopening
  // a month) the screen goes back to the live period.
  let liveSeen = $state('')
  $effect(() => {
    if (!settings.loaded || liveSeen === live) return
    liveSeen = live
    period = live
  })

  let closeError = $state('')
  async function closeMonth() {
    if (!confirm(`¿Cerrar ${monthName(live)}? Pasarás a trabajar con el dinero de ${monthName(shift(live, 1))}. Podrás reabrirlo.`)) return
    closeError = await settings.setClosedThrough(live)
  }
  async function reopenMonth() {
    closeError = await settings.setClosedThrough(shift(live, -2))
  }

  function showView(next: typeof view) {
    view = next
    drawer = false
  }

  // One sheet per currency: the whole screen shows a single currency at a time. A sheet exists
  // when the period has movements in it or the user added it (e.g. the currency of a trip).
  let sheet = $state<Currency>('')
  // Months the reports look back (Evolución and Buscar); the control sits next to the currency tabs.
  const monthRanges = [3, 6, 12]
  let months = $state(6)
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

<svelte:window onkeydown={(e) => e.key === 'Escape' && (drawer = false)} />

<div class="rd-shell" class:is-collapsed={collapsed} class:is-open={drawer}>
  <aside class="rd-sidebar" id="app-sidebar" aria-label="Navegación">
    <div class="rd-sidebar__head">
      <span class="rd-sidebar__brand"><Logo size={18} /><span class="rd-sidebar__label">Quri</span></span>
      <button class="rd-btn rd-btn--ghost rd-btn--sm rd-sidebar__toggle" aria-expanded={!collapsed} aria-label={collapsed ? 'Expandir menú' : 'Contraer menú'} onclick={toggleCollapsed}>
        <svg class="rd-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M15 5l-7 7 7 7" /></svg>
      </button>
    </div>

    <nav class="rd-sidebar__nav" aria-label="Periodo">
      {#if settings.loaded}
        <p class="rd-sidebar__group">Periodo</p>
        <div class="side-extra">
          <MonthPicker
            bind:value={period}
            today={live}
            onpick={() => (view = view === 'settings' ? 'movements' : view)}
          />
          <label class="check">
            <input type="checkbox" checked={projected} disabled={mode === 'plan'} onchange={(e) => (includePlanned = e.currentTarget.checked)} />
            Incluir lo planeado
          </label>
          {#if mode === 'current'}
            <button onclick={closeMonth}>Cerrar mes</button>
            {#if live > thisMonth}
              <button class="link neutral" onclick={reopenMonth}>Reabrir {monthName(shift(live, -1))}</button>
            {/if}
            {#if closeError}<p class="error" role="alert">{closeError}</p>{/if}
          {/if}
        </div>
      {/if}

      <div class="side-bottom">
        <p class="rd-sidebar__group">Sistema</p>
        <button class="rd-sidebar__link" class:is-active={view === 'settings'} aria-current={view === 'settings' ? 'page' : undefined} onclick={() => showView('settings')}>
          <svg class="rd-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 6h10M18 6h2M4 12h4M12 12h8M4 18h12M20 18h0" /><circle cx="16" cy="6" r="2" /><circle cx="10" cy="12" r="2" /><circle cx="18" cy="18" r="2" /></svg><span class="rd-sidebar__label">Ajustes</span>
        </button>
      </div>
    </nav>

    <div class="rd-sidebar__foot">
      <button class="rd-btn rd-btn--ghost rd-btn--sm" aria-label="Cambiar tema" onclick={toggleTheme}>
        <svg class="rd-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></svg>
      </button>
      {#if session.authRequired}
        <button class="rd-btn rd-btn--ghost rd-btn--sm rd-sidebar__label" onclick={() => session.logout()}>Salir</button>
      {/if}
    </div>
  </aside>

  <div class="rd-shell__main">
    <header class="rd-shell__bar">
      <button class="rd-btn rd-btn--ghost rd-btn--sm rd-shell__menu" aria-expanded={drawer} aria-controls="app-sidebar" aria-label="Abrir menú" onclick={() => (drawer = true)}>
        <svg class="rd-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 6h16M4 12h16M4 18h16" /></svg>
      </button>
      <div class="modes" role="group" aria-label="Sección">
        {#each ['movements', 'reports'] as const as v (v)}
          <button aria-pressed={view === v} onclick={() => showView(v)}>{viewTitle[v]}</button>
        {/each}
      </div>
    </header>

    <div class="rd-shell__content">
<main class="page">
  <header class="page-head">
    {#if view !== 'settings' && periodPattern.test(period)}
      <span class="eyebrow">{modeLabel[mode]} · {period}</span>
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
      <div class="segmented slider" role="group" aria-label="Moneda" style="--n: {sheets.length}; --i: {Math.max(0, sheets.indexOf(sheet))}">
        <span class="slider__thumb" aria-hidden="true"></span>
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
      {#if view === 'reports'}
        <div class="segmented sheets__months" role="group" aria-label="Meses de evolución">
          {#each monthRanges as r (r)}
            <button aria-pressed={months === r} onclick={() => (months = r)}>{r} meses</button>
          {/each}
        </div>
      {/if}
    </div>
  {/if}

  {#if loadError}
    <p class="error" role="alert">{loadError}</p>
  {:else if !valid}
    <p class="muted">Elige un mes en el calendario.</p>
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

  {#if valid}
    <div class="view" hidden={view !== 'reports'}>
      <Reports {period} currency={sheet} {projected} {months} summary={summary?.currencies?.find((x) => x.currency === sheet)} />
    </div>
    <div class="view" hidden={view === 'reports'}>
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
    </div>
  {/if}
  {/if}
</main>
    </div>
  </div>
  <div class="rd-shell__scrim" role="presentation" onclick={() => (drawer = false)}></div>
</div>
