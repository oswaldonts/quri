<script lang="ts">
  import { api, apiErrorMessage, connectionError, type Schemas } from './api'
  import { shift } from './dates'
  import { groupNames } from './groups'
  import { compareAmounts, formatMoney, type Currency } from './money'

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
</script>

<section class="panel" aria-labelledby="reports-title">
  <h2 id="reports-title">Reportes de {period}</h2>
  {#if !summary}
    <p class="muted">Sin movimientos en {currency} en este periodo.</p>
  {:else}
    {#if groups.length}
      <h3>Por grupo</h3>
      <ul class="bars">
        {#each groups as g (g.group)}
          <li>
            <span>{groupNames[g.group] ?? g.group}</span>
            <span class="bar-track"><span class="bar-fill" style:width="{share(g.spent, groups[0].spent)}%"></span></span>
            <span class="num">{formatMoney(g.spent, currency)}</span>
          </li>
        {/each}
      </ul>
    {/if}

    {#if tags.length}
      <h3>Por etiqueta</h3>
      <ul class="bars">
        {#each tags as t (t.tag)}
          <li>
            <span>#{t.tag} <span class="muted">({t.count})</span></span>
            <span class="bar-track"><span class="bar-fill" style:width="{share(t.spent, tags[0].spent)}%"></span></span>
            <span class="num">{formatMoney(t.spent, currency)}</span>
          </li>
        {/each}
      </ul>
    {/if}

    {#if items.length}
      <h3>Lo que más se gastó, por descripción</h3>
      <ul class="bars">
        {#each items as i (i.description)}
          <li>
            <span>{i.description || 'Sin descripción'} <span class="muted">({i.count})</span></span>
            <span class="bar-track"><span class="bar-fill" style:width="{share(i.spent, items[0].spent)}%"></span></span>
            <span class="num">{formatMoney(i.spent, currency)}</span>
          </li>
        {/each}
      </ul>
    {/if}
  {/if}

  <h3>Evolución</h3>
  <div class="segmented" role="group" aria-label="Meses">
    {#each ranges as r (r)}
      <button aria-pressed={months === r} onclick={() => (months = r)}>{r} meses</button>
    {/each}
  </div>
  {#if error}
    <p class="error" role="alert">{error}</p>
  {:else if trends}
    <div class="scroll-x"><table>
      <thead>
        <tr>
          <th>Mes</th>
          <th class="num">Ingresos</th>
          <th class="num">Gastado</th>
          <th class="num">Disponible</th>
        </tr>
      </thead>
      <tbody>
        {#each trends.periods ?? [] as p (p.period)}
          <tr>
            <td>{p.period}</td>
            <td class="num">{formatMoney(p.income, currency)}</td>
            <td class="num">{formatMoney(p.spent, currency)}</td>
            <td class="num">{formatMoney(p.available, currency)}</td>
          </tr>
        {/each}
      </tbody>
    </table></div>

    <label>
      Buscar categoría o etiqueta
      <input bind:value={filter} placeholder="Mercado, #dulces…" />
    </label>
    {#if rows.length}
      <div class="scroll-x"><table>
        <thead>
          <tr>
            <th>Elemento</th>
            <th class="num">Total</th>
            <th class="num">Promedio</th>
            {#each trends.periods ?? [] as p (p.period)}<th class="num">{monthLabel(p.period)}</th>{/each}
          </tr>
        </thead>
        <tbody>
          {#each rows as r (r.kind + r.name)}
            <tr>
              <td>{r.name} <span class="muted">{r.kind === 'Etiqueta' ? '' : '· cat.'}</span></td>
              <td class="num">{formatMoney(r.total, currency)}</td>
              <td class="num">{formatMoney(r.average, currency)}</td>
              {#each r.series as v, i (i)}<td class="num">{compareAmounts(v, '0') > 0 ? formatMoney(v, currency) : '—'}</td>{/each}
            </tr>
          {/each}
        </tbody>
      </table></div>
    {:else}
      <p class="muted">Nada que coincida en estos meses.</p>
    {/if}
  {:else}
    <p class="muted">Cargando…</p>
  {/if}
</section>
