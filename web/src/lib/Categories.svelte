<script lang="ts">
  import { onMount } from 'svelte'
  import { api, connectionError, failureMessage, type Schemas } from './api'
  import { groupNames, userGroups, type UserGroup } from './groups'

  // period is the month whose list of categories the checkboxes edit.
  let { period, onchange }: { period: string; onchange: () => void } = $props()

  let all = $state<Schemas['Category'][]>([])
  let inPeriod = $state(new Set<string>())
  let error = $state('')
  let busy = $state(false)

  let adding = $state(false)
  let name = $state('')
  let group = $state<UserGroup>('needs')
  let committed = $state(false)

  let editing = $state('')
  let editName = $state('')
  let editGroup = $state<UserGroup>('needs')
  let editCommitted = $state(false)

  const sections = $derived(
    Object.keys(groupNames)
      .map((key) => ({ key, items: all.filter((c) => c.group === key) }))
      .filter((s) => s.items.length > 0),
  )

  async function load() {
    try {
      const [cats, used] = await Promise.all([
        api.GET('/categories', { params: { query: { include_archived: true } } }),
        api.GET('/periods/{period}/categories', { params: { path: { period } } }),
      ])
      if (cats.data) all = cats.data
      else error = failureMessage(cats.response, cats.error)
      if (used.data) inPeriod = new Set((used.data.categories ?? []).map((c) => c.id))
    } catch {
      error = connectionError
    }
  }

  onMount(load)

  // Runs a write, then reloads the list; returns false (with the message shown) when it failed.
  async function run(call: () => Promise<{ error?: Schemas['ErrorModel']; response: Response }>): Promise<boolean> {
    error = ''
    busy = true
    try {
      const { error: failure, response } = await call()
      if (failure) {
        error = failureMessage(response, failure)
        return false
      }
      await load()
      onchange()
      return true
    } catch {
      error = connectionError
      return false
    } finally {
      busy = false
    }
  }

  async function create(event: SubmitEvent) {
    event.preventDefault()
    let id = ''
    const ok = await run(async () => {
      const res = await api.POST('/categories', { body: { name: name.trim(), group, committed } })
      id = res.data?.id ?? ''
      return res
    })
    if (!ok) return
    // A period with its own list wouldn't show the new category otherwise.
    if (id) await run(() => api.PUT('/periods/{period}/categories/{id}', { params: { path: { period, id } } }))
    adding = false
    name = ''
    committed = false
  }

  function edit(c: Schemas['Category']) {
    editing = c.id
    editName = c.name
    editGroup = c.group as UserGroup
    editCommitted = c.committed
  }

  async function save(event: SubmitEvent, id: string) {
    event.preventDefault()
    if (await run(() => api.PATCH('/categories/{id}', { params: { path: { id } }, body: { name: editName.trim(), group: editGroup, committed: editCommitted } }))) {
      editing = ''
    }
  }

  const setArchived = (c: Schemas['Category'], archived: boolean) =>
    run(() => api.PATCH('/categories/{id}', { params: { path: { id: c.id } }, body: { archived } }))

  const toggleInPeriod = (c: Schemas['Category'], use: boolean) =>
    run(() =>
      use
        ? api.PUT('/periods/{period}/categories/{id}', { params: { path: { period, id: c.id } } })
        : api.DELETE('/periods/{period}/categories/{id}', { params: { path: { period, id: c.id } } }),
    )
</script>

<section class="panel" aria-labelledby="categories-title">
  <div class="panel-head">
    <h2 id="categories-title">Categorías</h2>
    <button class="primary" aria-expanded={adding} onclick={() => (adding = !adding)}>
      {adding ? 'Cerrar' : 'Agregar categoría'}
    </button>
  </div>

  {#if adding}
    <form class="entry-form" onsubmit={create}>
      <div class="row wide">
        <label class="grow">
          Nombre
          <input bind:value={name} maxlength="50" placeholder="Mascotas" required />
        </label>
        <label>
          Grupo
          <select bind:value={group}>
            {#each userGroups as g (g)}<option value={g}>{groupNames[g]}</option>{/each}
          </select>
        </label>
      </div>
      <div class="wide actions">
        <label class="check">
          <input type="checkbox" bind:checked={committed} />
          Fija (se paga todos los meses)
        </label>
        <button class="primary" disabled={busy || !name.trim()}>Guardar categoría</button>
      </div>
    </form>
  {/if}

  {#if error}<p class="error" role="alert">{error}</p>{/if}

  <table>
    <thead>
      <tr>
        <th>Categoría</th>
        <th>Se usa en {period}</th>
        <th></th>
      </tr>
    </thead>
    {#each sections as s (s.key)}
      <tbody>
        <tr><th colspan="3" class="muted">{groupNames[s.key]}</th></tr>
        {#each s.items as c (c.id)}
          <tr>
            {#if editing === c.id}
              <td colspan="3">
                <form class="inline-edit" onsubmit={(e) => save(e, c.id)}>
                  <input bind:value={editName} maxlength="50" aria-label="Nombre de {c.name}" required />
                  <select bind:value={editGroup} aria-label="Grupo de {c.name}">
                    {#each userGroups as g (g)}<option value={g}>{groupNames[g]}</option>{/each}
                  </select>
                  <label class="check"><input type="checkbox" bind:checked={editCommitted} /> Fija</label>
                  <button class="primary" disabled={busy}>Guardar</button>
                  <button type="button" class="link neutral" onclick={() => (editing = '')}>Cancelar</button>
                </form>
              </td>
            {:else}
              <td>
                {c.name}
                {#if c.committed}<span class="badge">Fija</span>{/if}
                {#if c.archived}<span class="badge">Archivada</span>{/if}
              </td>
              <td>
                {#if c.system}
                  <span class="muted">Siempre</span>
                {:else}
                  <input
                    type="checkbox"
                    aria-label="Usar {c.name} en {period}"
                    checked={inPeriod.has(c.id)}
                    disabled={busy || c.archived}
                    onchange={(e) => toggleInPeriod(c, e.currentTarget.checked)}
                  />
                {/if}
              </td>
              <td class="num">
                {#if !c.system}
                  <button class="link neutral" aria-label="Editar {c.name}" onclick={() => edit(c)}>Editar</button>
                  <button class="link neutral" aria-label="{c.archived ? 'Restaurar' : 'Archivar'} {c.name}" onclick={() => setArchived(c, !c.archived)}>
                    {c.archived ? 'Restaurar' : 'Archivar'}
                  </button>
                {/if}
              </td>
            {/if}
          </tr>
        {/each}
      </tbody>
    {/each}
  </table>
  <p class="muted">
    Las categorías no se borran: se archivan, y dejan de ofrecerse en los periodos nuevos pero se conservan en el historial.
    Quitar una categoría de {period} no la quita de otros meses; si ese mes ya tiene gastos en ella, no se podrá quitar.
  </p>
</section>
