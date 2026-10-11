<script lang="ts">
  import { session } from './session.svelte'

  // Mirrors the API's minimum so the user gets the message without a round trip.
  const minLength = 8

  let password = $state('')
  let confirm = $state('')
  let error = $state('')
  let busy = $state(false)

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    if (password.length < minLength) {
      error = `La contraseña debe tener al menos ${minLength} caracteres.`
      return
    }
    if (password !== confirm) {
      error = 'Las contraseñas no coinciden.'
      return
    }
    busy = true
    error = (await session.setup(password)) ?? ''
    busy = false
  }
</script>

<main class="card">
  <h1>Bienvenido</h1>
  <p class="muted">Primer uso: elige la contraseña que protegerá tus datos.</p>
  <form onsubmit={submit}>
    <label>
      Contraseña
      <input type="password" bind:value={password} autocomplete="new-password" required />
    </label>
    <label>
      Repite la contraseña
      <input type="password" bind:value={confirm} autocomplete="new-password" required />
    </label>
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
    <button class="primary" disabled={busy || !password || !confirm}>Crear contraseña</button>
  </form>
</main>
