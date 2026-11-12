<script lang="ts">
  import Logo from './Logo.svelte'
  import { session } from './session.svelte'

  let password = $state('')
  let error = $state('')
  let busy = $state(false)

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    busy = true
    error = (await session.login(password)) ?? ''
    busy = false
  }
</script>

<main class="card">
  <h1 class="brand-title"><Logo size={28} />Quri</h1>
  <form onsubmit={submit}>
    <label>
      Contraseña
      <input type="password" bind:value={password} autocomplete="current-password" required />
    </label>
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
    <button class="primary" disabled={busy || !password}>Entrar</button>
  </form>
</main>
