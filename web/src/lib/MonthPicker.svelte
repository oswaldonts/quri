<script lang="ts">
  // Month picker: the Runner DS calendar's month layer (.rd-calendar__months), one year at a time.
  let {
    value = $bindable(''),
    min,
    max,
    today,
    locked = false,
  }: { value?: string; min?: string; max?: string; today?: string; locked?: boolean } = $props()

  let year = $state(new Date().getFullYear())
  $effect(() => {
    const y = Number(value.slice(0, 4))
    if (y) year = y
  })

  const months = Array.from({ length: 12 }, (_, m) => ({
    key: String(m + 1).padStart(2, '0'),
    name: new Date(2000, m, 1).toLocaleDateString('es', { month: 'long' }),
    label: new Date(2000, m, 1).toLocaleDateString('es', { month: 'short' }).replace('.', ''),
  }))
  const blocked = (p: string) => locked || (!!min && p < min) || (!!max && p > max)
  const prevOff = $derived(locked || (!!min && `${year - 1}-12` < min))
  const nextOff = $derived(locked || (!!max && `${year + 1}-01` > max))
</script>

<div class="rd-calendar month-picker" class:is-locked={locked}>
  <div class="rd-calendar__head">
    <button type="button" class="rd-calendar__nav" aria-label="Año anterior" disabled={prevOff} onclick={() => year--}>
      <svg class="rd-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M15 5l-7 7 7 7" /></svg>
    </button>
    <span class="rd-calendar__title" aria-live="polite">{year}</span>
    <button type="button" class="rd-calendar__nav" aria-label="Año siguiente" disabled={nextOff} onclick={() => year++}>
      <svg class="rd-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 5l7 7-7 7" /></svg>
    </button>
  </div>
  <div class="rd-calendar__months" role="group" aria-label="Meses de {year}">
    {#each months as m (m.key)}
      {@const p = `${year}-${m.key}`}
      <button
        type="button"
        class="rd-calendar__month"
        class:is-selected={p === value}
        class:is-today={p === today}
        aria-pressed={p === value}
        aria-label="{m.name} {year}"
        disabled={blocked(p)}
        onclick={() => (value = p)}
      >
        {m.label}
      </button>
    {/each}
  </div>
</div>
