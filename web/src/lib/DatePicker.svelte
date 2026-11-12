<script lang="ts">
  import { tick } from 'svelte'
  import { today } from './dates'

  // Date field drawn with the Runner DS calendar (.rd-calendar). The value is YYYY-MM-DD.
  let { value = $bindable(''), label, id }: { value?: string; label?: string; id?: string } = $props()

  const pad = (n: number) => String(n).padStart(2, '0')
  const fmt = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
  const parse = (s: string) => {
    const [y, m, d] = s.split('-').map(Number)
    return new Date(y, m - 1, d)
  }
  const valid = (s: string) => /^\d{4}-\d{2}-\d{2}$/.test(s)

  let open = $state(false)
  let pos = $state({ top: 0, left: 0 })
  let view = $state(new Date())
  let layer = $state<'days' | 'months'>('days')
  let focused = $state('')
  let wrap = $state<HTMLDivElement>()
  let pop = $state<HTMLDivElement>()
  let trigger = $state<HTMLButtonElement>()

  const title = $derived(view.toLocaleDateString('es', { month: 'long', year: 'numeric' }))
  const weekdays = ['L', 'M', 'X', 'J', 'V', 'S', 'D']
  const weeks = $derived.by(() => {
    const first = new Date(view.getFullYear(), view.getMonth(), 1)
    const start = new Date(first.getFullYear(), first.getMonth(), 1 - ((first.getDay() + 6) % 7))
    return Array.from({ length: 6 }, (_, w) =>
      Array.from({ length: 7 }, (_, d) => {
        const day = new Date(start.getFullYear(), start.getMonth(), start.getDate() + w * 7 + d)
        return { key: fmt(day), n: day.getDate(), outside: day.getMonth() !== view.getMonth() }
      }),
    )
  })

  async function show() {
    const base = valid(value) ? parse(value) : new Date()
    view = new Date(base.getFullYear(), base.getMonth(), 1)
    focused = fmt(base)
    layer = 'days'
    open = true
    await tick()
    place()
    focusDay(focused)
  }

  // The popup lives in <body> (outside the form's label and styles), so it is placed by hand.
  function place() {
    const t = trigger?.getBoundingClientRect()
    if (!t || !pop) return
    const h = pop.offsetHeight
    const w = pop.offsetWidth
    const top = t.bottom + 4 + h > innerHeight && t.top > h + 4 ? t.top - h - 4 : t.bottom + 4
    pos = { top, left: Math.max(8, Math.min(t.left, innerWidth - w - 8)) }
  }

  function portal(node: HTMLElement) {
    document.body.appendChild(node)
    return { destroy: () => node.remove() }
  }

  function close(refocus = false) {
    open = false
    if (refocus) trigger?.focus()
  }

  function focusDay(key: string) {
    pop?.querySelector<HTMLButtonElement>(`[data-d="${key}"]`)?.focus()
  }

  function pick(key: string) {
    value = key
    close(true)
  }

  async function goTo(day: Date) {
    focused = fmt(day)
    view = new Date(day.getFullYear(), day.getMonth(), 1)
    await tick()
    focusDay(focused)
  }

  function month(delta: number) {
    view = new Date(view.getFullYear(), view.getMonth() + (layer === 'months' ? delta * 12 : delta), 1)
  }

  const monthNames = Array.from({ length: 12 }, (_, m) => ({
    m,
    name: new Date(2000, m, 1).toLocaleDateString('es', { month: 'long' }),
    label: new Date(2000, m, 1).toLocaleDateString('es', { month: 'short' }).replace('.', ''),
  }))

  async function pickMonth(m: number) {
    view = new Date(view.getFullYear(), m, 1)
    layer = 'days'
    focused = fmt(view)
    await tick()
    focusDay(focused)
  }

  function keydown(e: KeyboardEvent) {
    const day = parse(focused)
    const step: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 }
    if (e.key in step) {
      e.preventDefault()
      goTo(new Date(day.getFullYear(), day.getMonth(), day.getDate() + step[e.key]))
    } else if (e.key === 'PageUp' || e.key === 'PageDown') {
      e.preventDefault()
      goTo(new Date(day.getFullYear(), day.getMonth() + (e.key === 'PageUp' ? -1 : 1), day.getDate()))
    }
  }
</script>

<svelte:window
  onclick={(e) => open && !wrap?.contains(e.target as Node) && !pop?.contains(e.target as Node) && close()}
  onresize={() => open && place()}
  onscrollcapture={(e) => open && !pop?.contains(e.target as Node) && close()}
  onkeydown={(e) => {
    if (open && e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      close(true)
    }
  }}
/>

<div class="rd-combo date-picker" class:is-open={open} bind:this={wrap}>
  <button
    type="button"
    {id}
    class="rd-select rd-combo__btn"
    aria-haspopup="dialog"
    aria-expanded={open}
    aria-label={label}
    bind:this={trigger}
    onclick={() => (open ? close() : show())}
  >
    {value || ' '}
  </button>
  {#if open}
    <div class="date-pop" style="top: {pos.top}px; left: {pos.left}px" role="dialog" aria-label="Elegir fecha" bind:this={pop} use:portal>
      <div class="rd-calendar">
        <div class="rd-calendar__head">
          <button type="button" class="rd-calendar__nav" aria-label={layer === 'months' ? 'Año anterior' : 'Mes anterior'} onclick={() => month(-1)}>
            <svg class="rd-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M15 5l-7 7 7 7" /></svg>
          </button>
          <button
            type="button"
            class="rd-calendar__title rd-calendar__title--btn"
            aria-live="polite"
            aria-label={layer === 'months' ? `${view.getFullYear()}, volver a los días` : `${title}, elegir mes`}
            onclick={() => (layer = layer === 'days' ? 'months' : 'days')}
          >
            {layer === 'months' ? view.getFullYear() : title}
            {#if layer === 'days'}<svg class="rd-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 9l7 7 7-7" /></svg>{/if}
          </button>
          <button type="button" class="rd-calendar__nav" aria-label={layer === 'months' ? 'Año siguiente' : 'Mes siguiente'} onclick={() => month(1)}>
            <svg class="rd-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 5l7 7-7 7" /></svg>
          </button>
        </div>
        {#if layer === 'months'}
          <div class="rd-calendar__months" role="group" aria-label="Meses de {view.getFullYear()}">
            {#each monthNames as n (n.m)}
              <button
                type="button"
                class="rd-calendar__month"
                class:is-selected={n.m === view.getMonth()}
                class:is-today={n.m === new Date().getMonth() && view.getFullYear() === new Date().getFullYear()}
                aria-label="{n.name} {view.getFullYear()}"
                onclick={() => pickMonth(n.m)}
              >
                {n.label}
              </button>
            {/each}
          </div>
        {:else}
        <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
        <table class="rd-calendar__grid" role="grid" onkeydown={keydown}>
          <thead>
            <tr>{#each weekdays as w (w)}<th scope="col">{w}</th>{/each}</tr>
          </thead>
          <tbody>
            {#each weeks as row, i (i)}
              <tr>
                {#each row as d (d.key)}
                  <td>
                    <button
                      type="button"
                      class="rd-calendar__day"
                      class:is-outside={d.outside}
                      class:is-today={d.key === today()}
                      class:is-selected={d.key === value}
                      data-d={d.key}
                      tabindex={d.key === focused ? 0 : -1}
                      aria-pressed={d.key === value}
                      onclick={() => pick(d.key)}
                    >
                      {d.n}
                    </button>
                  </td>
                {/each}
              </tr>
            {/each}
          </tbody>
        </table>
        {/if}
        <div class="rd-calendar__foot">
          <span class="rd-calendar__sel">{value}</span>
          <button type="button" class="rd-btn rd-btn--ghost rd-btn--sm" onclick={() => pick(today())}>Hoy</button>
        </div>
      </div>
    </div>
  {/if}
</div>
