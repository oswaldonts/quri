<script lang="ts">
  let { size = 20 }: { size?: number } = $props()
</script>

<svg class="logo" viewBox="-480 -300 960 600" width={size * 1.6} height={size} fill="currentColor" aria-hidden="true">
  <polygon points="-279,-222 0,-222 0,0 -462,0" />
  <polygon points="-462,0 -239,0 0,288 -222,288" />
  <polygon points="0,-288 222,-288 462,0 239,0" />
  <polygon points="0,0 462,0 279,222 0,222" />
</svg>
