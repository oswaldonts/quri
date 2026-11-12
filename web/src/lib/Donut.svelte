<script lang="ts">
  // Runner DS donut/pie (.rd-donut--pie): same geometry as ds.js, with a constant-width gap between sectors.
  export type Segment = { value: number; label: string; text: string }

  let { segments, series, active = $bindable(-1), label = '' }: {
    segments: Segment[]
    series: (i: number) => number
    active?: number
    label?: string
  } = $props()

  const GAP = 0.8
  const R = 50
  const TAU = Math.PI * 2

  const total = $derived(segments.reduce((sum, s) => sum + Math.max(0, s.value), 0))
  const visible = $derived(segments.filter((s) => s.value > 0).length)

  function edge(a: number, rho: number, isStart: boolean) {
    const d = GAP / 2
    const s = Math.sqrt(Math.max(rho * rho - d * d, 0))
    const nx = isStart ? -Math.sin(a) : Math.sin(a)
    const ny = isStart ? Math.cos(a) : -Math.cos(a)
    return `${(50 + s * Math.cos(a) + d * nx).toFixed(3)} ${(50 + s * Math.sin(a) + d * ny).toFixed(3)}`
  }

  // Pie: the inner radius is 0, so the sector closes at the centre, pushed out by the gap.
  function sector(a0: number, a1: number) {
    const d = GAP / 2
    const span = a1 - a0 - 2 * Math.asin(Math.min(d / R, 1))
    if (span <= 0) return ''
    const am = (a0 + a1) / 2
    const k = d / Math.sin((a1 - a0) / 2)
    return (
      `M${edge(a0, R, true)}A${R} ${R} 0 ${span > Math.PI ? 1 : 0} 1 ${edge(a1, R, false)}` +
      `L${(50 + k * Math.cos(am)).toFixed(3)} ${(50 + k * Math.sin(am)).toFixed(3)}Z`
    )
  }

  const paths = $derived.by(() => {
    let acc = 0
    return segments.map((s) => {
      if (s.value <= 0 || total <= 0) return ''
      const f = s.value / total
      const d = visible > 1 ? sector(acc * TAU, (acc + f) * TAU) : 'M50 0A50 50 0 1 1 50 100A50 50 0 1 1 50 0Z'
      acc += f
      return d
    })
  })
</script>

<div class="rd-donut rd-donut--pie" class:has-active={active >= 0} role="img" aria-label={label}>
  <svg class="rd-donut__svg" viewBox="0 0 100 100" aria-hidden="true">
    {#each segments as s, i (i)}
      {#if paths[i]}
        <path
          class="rd-donut__seg rd-series-{series(i)}"
          class:is-active={active === i}
          d={paths[i]}
          role="presentation"
          onmouseenter={() => (active = i)}
          onmouseleave={() => (active = -1)}
        >
          <title>{s.text}</title>
        </path>
      {/if}
    {/each}
  </svg>
</div>
