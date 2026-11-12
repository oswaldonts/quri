<script lang="ts">
  import { api, apiErrorMessage, connectionError, type Schemas } from './api'
  import Donut from './Donut.svelte'
  import { shift } from './dates'
  import { groupNames } from './groups'
  import { compareAmounts, formatMoney, sumAmounts, type Currency } from './money'

  let { period, currency, projected, months, summary }: {
    period: string
    currency: Currency
    projected: boolean
    months: number
    summary: Schemas['CurrencySummary'] | undefined
  } = $props()

  // Share of the biggest value, for the bar widths (display only).
  const share = (amount: string, max: string) => {
    const m = Number(max)
    return m > 0 ? Math.max(2, Math.round((Number(amount) / m) * 100)) : 0
  }

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

  type Row = { name: string; total: string }
  const rows = $derived.by<Row[]>(() => {
    if (!trends) return []
    const q = filter.trim().toLowerCase()
    return [
      ...(trends.expenses ?? []).map((e) => ({ name: e.name, total: e.total })),
      ...(trends.tags ?? []).map((t) => ({ name: `#${t.tag}`, total: t.total })),
    ]
      .filter((r) => compareAmounts(r.total, '0') > 0 && r.name.toLowerCase().includes(q))
      .sort((a, b) => compareAmounts(b.total, a.total))
  })
  const monthLabel = (p: string) => p.slice(2)

  const groups = $derived((summary?.groups ?? []).filter((g) => compareAmounts(g.spent, '0') > 0))
  const tags = $derived(summary?.tags ?? [])
  const items = $derived(summary?.top_items ?? [])

  // The DS has five series colours; past the fifth they repeat.
  const series = (i: number) => (i % 5) + 1
  const groupsTotal = $derived(sumAmounts(groups.map((g) => g.spent)))
  const percent = (amount: string, total: string) => (Number(total) > 0 ? Math.round((Number(amount) / Number(total)) * 100) : 0)
  const groupName = (g: string) => groupNames[g] ?? g
  const segments = $derived(
    groups.map((g) => ({
      value: Number(g.spent),
      label: groupName(g.group),
      text: `${groupName(g.group)}: ${formatMoney(g.spent, currency)} (${percent(g.spent, groupsTotal)}%)`,
    })),
  )
  let activeGroup = $state(-1)

  const periods = $derived(trends?.periods ?? [])
  const peak = $derived(
    periods.reduce((m, p) => Math.max(m, Number(p.income), Number(p.spent)), 0),
  )
  const height = (amount: string) => (peak > 0 ? Math.max(1, Math.round((Number(amount) / peak) * 100)) : 0)
</script>

{#snippet evolution()}
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
{/snippet}

<div class="reports">
  {#if !summary}
    <section class="panel">
      <p class="muted">Sin movimientos en {currency} en este periodo.</p>
    </section>
    {@render evolution()}
  {:else}
    <div class="reports-grid">
      {#if groups.length}
        <section class="panel panel--center" aria-labelledby="by-group">
          <h3 id="by-group">Por grupo</h3>
          <div class="donut-row">
            <Donut {segments} {series} bind:active={activeGroup} label="Reparto del gasto por grupo" />
            <ul class="rd-legend rd-legend--bars">
              {#each groups as g, i (g.group)}
                <li
                  class="rd-series-{series(i)}"
                  class:is-active={activeGroup === i}
                  onmouseenter={() => (activeGroup = i)}
                  onmouseleave={() => (activeGroup = -1)}
                >
                  <i></i>
                  <span>{groupName(g.group)}</span>
                  <span class="muted">{percent(g.spent, groupsTotal)}%</span>
                  <span class="num">{formatMoney(g.spent, currency)}</span>
                </li>
              {/each}
            </ul>
          </div>
        </section>
      {/if}

      {#if items.length}
        <section class="panel" aria-labelledby="top-items">
          <h3 id="top-items">Lo que más se gastó</h3>
          <ul class="rd-bars">
            {#each items as i (i.description)}
              <li>
                <span class="rd-bars__label">{i.description || 'Sin descripción'} ({i.count})</span>
                <span class="rd-bar"><span class="rd-bar__fill" style="--value:{share(i.spent, items[0].spent)}"></span></span>
                <span class="rd-bars__value">{formatMoney(i.spent, currency)}</span>
              </li>
            {/each}
          </ul>
        </section>
      {/if}

      {@render evolution()}

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
    </div>
  {/if}


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
              </tr>
            </thead>
            <tbody>
              {#each rows as r (r.name)}
                <tr>
                  <td>{r.name}</td>
                  <td class="num">{formatMoney(r.total, currency)}</td>
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
