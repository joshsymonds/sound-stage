<script lang="ts">
  import { hashHue } from "$lib/color";
  import type { Song } from "$lib/types";

  let {
    song,
    onqueue,
    id,
  }: { song: Song; onqueue: (song: Song) => void; id?: string } = $props();

  // Callers always render CoverTile in a `{#each ... (song.id)}` keyed
  // block, so a new song.id gets a fresh component instance (and thus a
  // fresh imgFailed) automatically — no manual reset needed here.
  let imgFailed = $state(false);

  const showImg = $derived(song.id !== "" && !imgFailed);

  const initials = $derived(
    song.artist
      .trim()
      .split(/\s+/)
      .slice(0, 2)
      .map((word) => word.charAt(0).toUpperCase())
      .join(""),
  );
</script>

<button class="cover-tile" type="button" {id} onclick={() => onqueue(song)}>
  <div class="cover">
    {#if showImg}
      <img
        src={`/api/library/${song.id}/thumb`}
        alt=""
        loading="lazy"
        onerror={() => (imgFailed = true)}
      />
    {:else}
      <div class="fallback" style={`background: hsl(${String(hashHue(song.artist))} 60% 35%)`}>
        <span class="initials">{initials}</span>
      </div>
    {/if}
  </div>
  <span class="title">{song.title}</span>
  <span class="artist">{song.artist}</span>
</button>

<style>
  .cover-tile {
    display: flex;
    flex-direction: column;
    gap: 2px;
    width: 100%;
    min-width: 0;
    padding: 0;
    background: none;
    border: none;
    text-align: left;
    font-family: var(--font-body);
    color: var(--color-text);
    cursor: pointer;
    min-height: 44px;
    content-visibility: auto;
    contain-intrinsic-size: auto 190px;
  }

  .cover {
    width: 100%;
    aspect-ratio: 1 / 1;
    border-radius: var(--radius-md);
    overflow: hidden;
    background: var(--color-surface);
    border: 1px solid var(--color-border-subtle);
    transition: border-color var(--transition-normal), box-shadow var(--transition-normal);
  }

  .cover-tile:hover .cover,
  .cover-tile:active .cover {
    border-color: var(--color-pink);
    box-shadow: var(--glow-pink);
  }

  .cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .fallback {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .initials {
    font-size: 1.5rem;
    font-weight: 700;
    color: var(--color-text);
    opacity: 0.85;
  }

  .title {
    margin-top: var(--space-xs);
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--color-text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .artist {
    font-size: 0.75rem;
    color: var(--color-text-dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
