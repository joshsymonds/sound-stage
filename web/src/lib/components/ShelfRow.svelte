<script lang="ts">
  import CoverTile from "$lib/components/CoverTile.svelte";
  import type { Song } from "$lib/types";

  let {
    label,
    songs,
    onqueue,
  }: { label: string; songs: Song[]; onqueue: (song: Song) => void } = $props();
</script>

{#if songs.length > 0}
  <div class="shelf">
    <div class="shelf-label">{label}</div>
    <div class="shelf-row">
      {#each songs as song (song.id)}
        <div class="tile">
          <CoverTile {song} {onqueue} />
        </div>
      {/each}
    </div>
  </div>
{/if}

<style>
  .shelf {
    margin-bottom: var(--space-lg);
  }

  .shelf-label {
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    color: var(--color-pink);
    text-shadow: var(--glow-text-pink);
    margin-bottom: var(--space-sm);
  }

  .shelf-row {
    display: flex;
    gap: var(--space-sm);
    overflow-x: auto;
    scroll-snap-type: x mandatory;
    -webkit-overflow-scrolling: touch;
    padding-bottom: var(--space-xs);
  }

  .tile {
    flex: 0 0 128px;
    width: 128px;
    scroll-snap-align: start;
  }
</style>
