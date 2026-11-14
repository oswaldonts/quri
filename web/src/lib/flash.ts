import { tick } from 'svelte'

// Scrolls to the row of an entry that was just saved, so the user sees where it landed.
export async function revealRow(id: string) {
  await tick()
  document.getElementById(`row-${id}`)?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
}
