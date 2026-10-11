<script lang="ts">
  import { onMount } from 'svelte'
  import Home from './lib/Home.svelte'
  import Login from './lib/Login.svelte'
  import { session } from './lib/session.svelte'
  import Setup from './lib/Setup.svelte'

  onMount(() => session.check())
</script>

{#if session.state === 'loading'}
  <p class="center muted">Cargando…</p>
{:else if session.state === 'setup'}
  <Setup />
{:else if session.state === 'login'}
  <Login />
{:else if session.state === 'error'}
  <main class="card">
    <h1>Sin conexión</h1>
    <p class="muted">No se pudo conectar con el servidor.</p>
    <button class="primary" onclick={() => session.check()}>Reintentar</button>
  </main>
{:else}
  <Home />
{/if}
