<script lang="ts">
  import { api, apiErrorMessage, connectionError, type Schemas } from './api'
  import { shift } from './dates'
  import { groupNames } from './groups'
  import { compareAmounts, formatMoney, sumAmounts, type Currency } from './money'

  let { period, currency, projected, summary }: {
    period: string
    currency: Currency
    projected: boolean
    summary: Schemas['CurrencySummary'] | undefined
  } = $props()

  // Share of the biggest value, for the bar widths (display only).
  const share = (amount: string, max: string) => {
    const m = Number(max)
    return m > 0 ? Math.max(2, Math.round((Number(amount) / m) * 100)) : 0
  }

  const ranges = [3, 6, 12]
  let months = $state(6)
  let trends = $state<Schemas['Trends'] | null>(null)
  let error = $state('')
  let filter = $state('')

  $effect(() => {
    const controller = new AbortController()
    error = ''
    api
      .GET('/trends', {
        params: {
          query: { from: shift(period, -(months - 1)), to: period, currency, view: projected ? 'projected' : 'confirmed' },
        },
        signal: controller.signal,
      })
      .then(({ data, error: err, response }) => {
        if (data) trends = data
        else if (response.status !== 401 && response.status !== 428) error = apiErrorMessage(err)
      })
      .catch(() => {
        if (!controller.signal.aborted) error = connectionError
      })
    return () => controller.abort()
  })

  type Row = { kind: 'Categoría' | 'Etiqueta'; name: string; total: string; average: string; series: string[] }
  const rows = $derived.by<Row[]>(() => {
    if (!trends) return []
    const q = filter.trim().toLowerCase()
    return [
      ...(trends.expenses ?? []).map((e) => ({ kind: 'Categoría' as const, name: e.name, total: e.total, average: e.average, series: e.series ?? [] })),
      ...(trends.tags ?? []).map((t) => ({ kind: 'Etiqueta' as const, name: `#${t.tag}`, total: t.total, average: t.average, series: t.series ?? [] })),
    ]
      .filter((r) => compareAmounts(r.total, '0') > 0 && r.name.toLowerCase().includes(q))
      .sort((a, b) => compareAmounts(b.total, a.total))
  })
  const monthLabel = (p: string) => p.slice(2)

  const groups = $derived((summary?.groups ?? []).filter((g) => compareAmounts(g.spent, '0') > 0))
  const tags = $derived(summary?.tags ?? [])
  const items = $derived(summary?.top_items ?? [])

  const colors = ['var(--rd-accent-fill)', 'var(--rd-info)', 'var(--rd-warning)', 'var(--rd-success)', 'var(--rd-danger)', 'var(--rd-text-dim)']
  const groupsTotal = $derived(sumAmounts(groups.map((g) => g.spent)))
  const percent = (amount: string, total: string) => (Number(total) > 0 ? Math.round((Number(amount) / Number(total)) * 100) : 0)

  const periods = $derived(trends?.periods ?? [])
  const peak = $derived(
    periods.reduce((m, p) => Math.max(m, Number(p.income), Number(p.spent)), 0),
  )
  const height = (amount: string) => (peak > 0 ? Math.max(1, Math.round((Number(amount) / peak) * 100)) : 0)
  const rowPeak = (series: string[]) => series.reduce((m, v) => Math.max(m, Number(v)), 0)
</script>

<div class="reports">
  <div class="reports-head">
    <h2 id="reports-title">Reportes · {period}</h2>
    <div class="segmented" role="group" aria-label="Meses de evolución">
      {#each ranges as r (r)}
        <button aria-pressed={months === r} onclick={() => (months = r)}>{r} meses</button>
      {/each}
    </div>
  </div>

  {#if !summary}
    <section class="panel">
      <p class="muted">Sin movimientos en {currency} en este periodo.</p>
    </section>
  {:else}
    <div class="reports-grid">
      {#if groups.length}
        <section class="panel wide" aria-labelledby="by-group">
          <h3 id="by-group">Por grupo</h3>
          <div class="stack" role="img" aria-label="Reparto del gasto por grupo">
            {#each groups as g, i (g.group)}
              <span style:flex-grow={Number(g.spent)} style:background={colors[i % colors.length]} title={groupNames[g.group] ?? g.group}></span>
            {/each}
          </div>
          <ul class="legend">
            {#each groups as g, i (g.group)}
              <li>
                <i style:background={colors[i % colors.length]}></i>
                <span>{groupNames[g.group] ?? g.group}</span>
                <span class="muted">{percent(g.spent, groupsTotal)}%</span>
                <span class="num">{formatMoney(g.spent, currency)}</span>
              </li>
            {/each}
          </ul>
        </section>
      {/if}

      {#if tags.length}
        <section class="panel" aria-labelledby="by-tag">
          <h3 id="by-tag">Por etiqueta</h3>
          <ul class="tag-list">
            {#each tags as t (t.tag)}
              <li style:--w="{share(t.spent, tags[0].spent)}%">
                <b>#{t.tag}</b>
                <span class="muted">×{t.count}</span>
                <span class="num">{formatMoney(t.spent, currency)}</span>
              </li>
            {/each}
          </ul>
        </section>
      {/if}

      {#if items.length}
        <section class="panel" aria-labelledby="top-items">
          <h3 id="top-items">Lo que más se gastó</h3>
          <ol class="ranked">
            {#each items as i, n (i.description)}
              <li>
                <span class="rank">{String(n + 1).padStart(2, '0')}</span>
                <span class="what">{i.description || 'Sin descripción'} <span class="muted">({i.count})</span></span>
                <span class="num">{formatMoney(i.spent, currency)}</span>
                <span class="bar-track"><span class="bar-fill" style:width="{share(i.spent, items[0].spent)}%"></span></span>
              </li>
            {/each}
          </ol>
        </section>
      {/if}
    </div>
  {/if}

  <section class="panel" aria-labelledby="evolution">
    <div class="panel-head">
      <h3 id="evolution">Evolución</h3>
      <span class="key"><i class="income"></i>Ingresos</span>
      <span class="key"><i class="spent"></i>Gastado</span>
    </div>
    {#if error}
      <p class="error" role="alert">{error}</p>
    {:else if trends}
      <div class="columns">
        {#each periods as p (p.period)}
          <div class="column" title="{p.period}: ingresos {formatMoney(p.income, currency)}, gastado {formatMoney(p.spent, currency)}, disponible {formatMoney(p.available, currency)}">
            <div class="column-bars">
              <span class="income" style:height="{height(p.income)}%"></span>
              <span class="spent" style:height="{height(p.spent)}%"></span>
            </div>
            <span class="column-label">{monthLabel(p.period)}</span>
          </div>
        {/each}
      </div>
      <div class="rd-accordion">
        <details>
          <summary>Ver tabla</summary>
          <div class="rd-accordion__body scroll-x">
            <table>
              <thead>
                <tr>
                  <th>Mes</th>
                  <th class="num">Ingresos</th>
                  <th class="num">Gastado</th>
                  <th class="num">Disponible</th>
                </tr>
              </thead>
              <tbody>
                {#each periods as p (p.period)}
                  <tr>
                    <td>{p.period}</td>
                    <td class="num">{formatMoney(p.income, currency)}</td>
                    <td class="num">{formatMoney(p.spent, currency)}</td>
                    <td class="num">{formatMoney(p.available, currency)}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </details>
      </div>
    {:else}
      <p class="muted">Cargando…</p>
    {/if}
  </section>

  <section class="panel" aria-labelledby="search">
    <h3 id="search">Buscar categoría o etiqueta</h3>
    <label class="search">
      <span class="rd-sr-only">Buscar categoría o etiqueta</span>
      <input bind:value={filter} placeholder="Mercado, #dulces…" />
    </label>
    {#if trends}
      {#if rows.length}
        <div class="scroll-x">
          <table>
            <thead>
              <tr>
                <th>Elemento</th>
                <th class="num">Total</th>
                <th class="num">Promedio</th>
                <th>Últimos {periods.length} meses</th>
              </tr>
            </thead>
            <tbody>
              {#each rows as r (r.kind + r.name)}
                <tr>
                  <td>{r.name} <span class="muted">{r.kind === 'Etiqueta' ? '' : '· cat.'}</span></td>
                  <td class="num">{formatMoney(r.total, currency)}</td>
                  <td class="num">{formatMoney(r.average, currency)}</td>
                  <td>
                    <span class="spark">
                      {#each r.series as v, i (i)}
                        <i
                          style:height="{Number(v) > 0 ? Math.max(8, Math.round((Number(v) / rowPeak(r.series)) * 100)) : 0}%"
                          title="{periods[i]?.period}: {formatMoney(v, currency)}"
                        ></i>
                      {/each}
                    </span>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else}
        <p class="muted">Nada que coincida en estos meses.</p>
      {/if}
    {/if}
  </section>
</div>
