<script lang="ts">
  import { hashHue } from "$lib/color";

  let { name, showName = false }: { name: string; showName?: boolean } =
    $props();

  const initials = $derived(
    name
      .trim()
      .split(/\s+/)
      .slice(0, 2)
      .map((word) => word.charAt(0).toUpperCase())
      .join(""),
  );
</script>

<span class="guest-chip-wrap">
  <span
    class="guest-chip"
    style={`background: hsl(${String(hashHue(name))} 60% 40%)`}
  >
    {initials}
  </span>
  {#if showName}
    <span class="guest-chip-name">{name}</span>
  {/if}
</span>

<style>
  .guest-chip-wrap {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .guest-chip {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    flex-shrink: 0;
    border-radius: var(--radius-full);
    color: #fff;
    font-size: 0.6875rem;
    font-weight: 700;
    line-height: 1;
  }

  .guest-chip-name {
    font-size: 0.8125rem;
    color: var(--color-text);
  }
</style>
