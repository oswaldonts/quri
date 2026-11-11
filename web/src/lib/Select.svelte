<script lang="ts">
  // Runner DS combo (.rd-combo): a button plus a listbox, with the keyboard behaviour of ds.js.
  type Option = { value: string | number; label: string; disabled?: boolean }

  let {
    value = $bindable(''),
    options,
    onchange,
    disabled = false,
    placeholder = '',
    label,
    id,
  }: {
    value?: string | number
    options: Option[]
    onchange?: (value: string | number) => void
    disabled?: boolean
    placeholder?: string
    label?: string
    id?: string
  } = $props()

  const uid = $props.id()
  let open = $state(false)
  let up = $state(false)
  let active = $state(-1)
  let buffer = ''
  let bufferTimer: ReturnType<typeof setTimeout> | undefined
  let wrap = $state<HTMLDivElement>()
  let list = $state<HTMLUListElement>()

  const selected = $derived(options.findIndex((o) => o.value === value))
  const shown = $derived(selected >= 0 ? options[selected].label : placeholder)

  function show() {
    if (disabled) return
    open = true
    up = false
    active = selected
    requestAnimationFrame(() => {
      const below = list?.getBoundingClientRect().bottom ?? 0
      const room = wrap?.getBoundingClientRect().top ?? 0
      if (below > innerHeight && room > (list?.offsetHeight ?? 0)) up = true
      list?.children[active]?.scrollIntoView({ block: 'nearest' })
    })
  }

  function close() {
    open = false
    active = -1
  }

  function choose(i: number) {
    const option = options[i]
    if (!option || option.disabled) return
    const changed = option.value !== value
    value = option.value
    close()
    if (changed) onchange?.(option.value)
  }

  function move(dir: number, from: number) {
    let i = from
    do i += dir
    while (i >= 0 && i < options.length && options[i].disabled)
    if (i >= 0 && i < options.length) {
      active = i
      list?.children[i]?.scrollIntoView({ block: 'nearest' })
    }
  }

  function keydown(e: KeyboardEvent) {
    const k = e.key
    if (k === 'ArrowDown' || k === 'ArrowUp') {
      e.preventDefault()
      if (!open) show()
      else move(k === 'ArrowDown' ? 1 : -1, active)
    } else if (k === 'Home' || k === 'End') {
      if (!open) return
      e.preventDefault()
      if (k === 'Home') move(1, -1)
      else move(-1, options.length)
    } else if (k === 'Enter' || k === ' ') {
      e.preventDefault()
      if (open) choose(active)
      else show()
    } else if (k === 'Escape') {
      if (open) {
        e.preventDefault()
        e.stopPropagation()
        close()
      }
    } else if (k === 'Tab') {
      close()
    } else if (k.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
      buffer += k.toLowerCase()
      clearTimeout(bufferTimer)
      bufferTimer = setTimeout(() => (buffer = ''), 600)
      const from = open ? active : selected
      const order = [...options.keys()]
      const next = [...order.slice(from + 1), ...order.slice(0, from + 1)].find(
        (i) => !options[i].disabled && options[i].label.toLowerCase().startsWith(buffer),
      )
      if (next !== undefined) {
        if (open) {
          active = next
          list?.children[next]?.scrollIntoView({ block: 'nearest' })
        } else choose(next)
      }
    }
  }
</script>

<div class="rd-combo" class:is-open={open} class:is-up={up} bind:this={wrap}>
  <button
    type="button"
    {id}
    class="rd-select rd-combo__btn"
    role="combobox"
    aria-haspopup="listbox"
    aria-expanded={open}
    aria-controls="lb-{uid}"
    aria-label={label}
    aria-activedescendant={open && active >= 0 ? `opt-${uid}-${active}` : undefined}
    {disabled}
    onclick={(e) => {
      if (e.detail === 0) return
      if (open) close()
      else show()
    }}
    onblur={close}
    onkeydown={keydown}
  >
    {shown || ' '}
  </button>
  <ul class="rd-combo__list" role="listbox" id="lb-{uid}" hidden={!open} bind:this={list}>
    {#each options as o, i (o.value)}
      <li
        id="opt-{uid}-{i}"
        role="option"
        aria-selected={i === selected}
        aria-disabled={o.disabled ? 'true' : undefined}
        class:is-active={i === active}
        onmousedown={(e) => {
          e.preventDefault()
          choose(i)
        }}
      >
        {o.label}
      </li>
    {/each}
  </ul>
</div>
