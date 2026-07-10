<script lang="ts">
  import type { USDBResult } from "$lib/api";
  import CoverTile from "$lib/components/CoverTile.svelte";
  import FastScrollRail from "$lib/components/FastScrollRail.svelte";
  import ShelfRow from "$lib/components/ShelfRow.svelte";
  import SongCard from "$lib/components/SongCard.svelte";
  import { byDecade, duets, genreShelf, recentlyAdded } from "$lib/shelves";
  import type { Song } from "$lib/types";
  import { SvelteMap, SvelteSet } from "svelte/reactivity";

  let {
    songs,
    value,
    searching,
    loadingSongs,
    dedupedUSDB,
    downloadingIds,
    oninput,
    onqueue,
    ondownload,
  }: {
    songs: Song[];
    value: string;
    searching: boolean;
    loadingSongs: boolean;
    dedupedUSDB: USDBResult[];
    downloadingIds: Set<number>;
    oninput: (value: string) => void;
    onqueue: (song: Song) => void;
    ondownload: (result: USDBResult) => void;
  } = $props();

  // Must match +page.svelte's SEARCH_MIN_CHARS — that copy gates when the
  // owning component fires a USDB search; this copy only gates which
  // presentational branch (search results vs. full library) is shown.
  const SEARCH_MIN_CHARS = 2;

  // Library filter is client-side and instant — no debounce. Songs is at most
  // a few thousand entries; substring match across title + artist is cheap.
  const filteredSongs = $derived.by(() => {
    const q = value.trim().toLowerCase();
    if (q.length < SEARCH_MIN_CHARS) return songs;
    return songs.filter(
      (s) => s.title.toLowerCase().includes(q) || s.artist.toLowerCase().includes(q),
    );
  });
  const isSearching = $derived(value.trim().length >= SEARCH_MIN_CHARS);

  // Fast-scroll-rail bucketing: an artist's initial, or "#" for anything
  // that doesn't start with a plain A-Z letter.
  function initialLetter(song: Song): string {
    const first = song.artist.trim().charAt(0).toUpperCase();
    return /^[A-Z]$/.test(first) ? first : "#";
  }

  // "#" isn't safe inside a CSS id selector unescaped, so anchor ids spell
  // it out instead of embedding the character directly.
  function anchorSlug(letter: string): string {
    return letter === "#" ? "hash" : letter;
  }

  const recentSongs = $derived(recentlyAdded(songs));
  const decadeShelves = $derived(byDecade(songs));
  const duetSongs = $derived(duets(songs));
  const popSongs = $derived(genreShelf(songs, "pop"));
  const rockSongs = $derived(genreShelf(songs, "rock"));
  const soundtrackSongs = $derived(genreShelf(songs, "soundtrack"));

  // Hoisted: constructing collation options inside a comparator over the
  // full library (~2.6k songs) visibly stutters phone hardware at Browse-open.
  const crateCollator = new Intl.Collator(undefined, { sensitivity: "base" });

  const sortedSongs = $derived(
    [...songs].sort(
      (a, b) => crateCollator.compare(a.artist, b.artist) || crateCollator.compare(a.title, b.title),
    ),
  );

  const presentLetters = $derived(new SvelteSet(sortedSongs.map((song) => initialLetter(song))));

  // First song per letter (in crate order) gets a DOM id so the fast-scroll
  // rail can jump straight to it.
  const anchorIdsBySongId = $derived.by(() => {
    const seen = new SvelteSet<string>();
    const anchors = new SvelteMap<string, string>();
    for (const song of sortedSongs) {
      const letter = initialLetter(song);
      if (!seen.has(letter)) {
        seen.add(letter);
        anchors.set(song.id, `crate-${anchorSlug(letter)}`);
      }
    }
    return anchors;
  });

  function jumpToLetter(letter: string): void {
    document.querySelector(`#crate-${anchorSlug(letter)}`)?.scrollIntoView({ block: "start" });
  }
</script>

<div class="section">
  <div class="search-bar">
    <input
      type="search"
      class="search-input"
      placeholder="Search by title or artist…"
      {value}
      oninput={(inputEvent) => oninput(inputEvent.currentTarget.value)}
    />
    {#if searching}
      <span class="search-spinner" aria-label="Searching">…</span>
    {/if}
  </div>

  {#if isSearching}
    <div class="section-head" style="margin-top: var(--space-md);">
      <div class="section-label">Results</div>
      <div class="section-sub">
        {filteredSongs.length > 0
          ? "Library plays instantly · USDB downloads on tap"
          : "Tap a USDB result to download (~30s) and queue"}
      </div>
    </div>
    {#if filteredSongs.length === 0 && dedupedUSDB.length === 0 && !searching}
      <div class="empty-prompt">
        <p>No matches for &ldquo;{value}&rdquo;.</p>
      </div>
    {:else}
      {#if filteredSongs.length > 0}
        <div class="crate-grid">
          {#each filteredSongs as song (song.id)}
            <CoverTile {song} {onqueue} />
          {/each}
        </div>
      {/if}
      <div class="list">
        {#each dedupedUSDB as result (result.id)}
          <SongCard
            title={result.title}
            artist={result.artist}
            coverUrl={`/api/usdb/cover/${String(result.id)}`}
            onclick={() => ondownload(result)}
          />
          {#if downloadingIds.has(result.id)}
            <div class="download-status">Downloading…</div>
          {/if}
        {/each}
        {#if searching && dedupedUSDB.length === 0}
          <div class="empty-prompt"><p>Searching USDB…</p></div>
        {/if}
      </div>
    {/if}
  {:else if loadingSongs}
    <div class="empty-prompt"><p>Loading…</p></div>
  {:else if songs.length === 0}
    <div class="empty-prompt">
      <p>Nothing downloaded yet. Search above to grab a song.</p>
    </div>
  {:else}
    <ShelfRow label="Recently Added" songs={recentSongs} {onqueue} />
    {#each decadeShelves as decade (decade.label)}
      <ShelfRow label={decade.label} songs={decade.songs} {onqueue} />
    {/each}
    <ShelfRow label="Duets" songs={duetSongs} {onqueue} />
    <ShelfRow label="Pop" songs={popSongs} {onqueue} />
    <ShelfRow label="Rock" songs={rockSongs} {onqueue} />
    <ShelfRow label="Soundtracks" songs={soundtrackSongs} {onqueue} />

    <div class="section-head" style="margin-top: var(--space-md);">
      <div class="section-label">All Songs</div>
    </div>
    <div class="crate-grid">
      {#each sortedSongs as song (song.id)}
        <CoverTile {song} {onqueue} id={anchorIdsBySongId.get(song.id)} />
      {/each}
    </div>
    <FastScrollRail letters={presentLetters} onjump={jumpToLetter} />
  {/if}
</div>

<style>
  .section {
    padding: var(--space-md) var(--space-lg);
  }

  .section-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-sm);
    margin-bottom: var(--space-sm);
  }

  .section-label {
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    color: var(--color-pink);
    text-shadow: var(--glow-text-pink);
    margin-bottom: var(--space-sm);
  }

  .section-head .section-label {
    margin-bottom: 0;
  }

  .section-sub {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    letter-spacing: 0.02em;
  }

  .search-spinner {
    display: inline-flex;
    align-items: center;
    color: var(--color-text-muted);
    font-size: 1.25rem;
    padding: 0 var(--space-xs);
  }

  .list {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
  }

  .crate-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: var(--space-sm);
    margin-bottom: var(--space-lg);
  }

  .empty-prompt {
    padding: var(--space-lg);
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-md);
  }

  .empty-prompt p {
    color: var(--color-text-muted);
    font-size: 0.875rem;
  }

  .search-bar {
    display: flex;
    gap: var(--space-sm);
    margin-bottom: var(--space-md);
  }

  .search-input {
    flex: 1;
    padding: 10px 16px;
    background: var(--color-surface);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-md);
    color: var(--color-text);
    font-family: var(--font-body);
    font-size: 0.875rem;
    outline: none;
    transition: border-color var(--transition-normal), box-shadow var(--transition-normal);
  }

  .search-input:focus {
    border-color: var(--color-pink);
    box-shadow: var(--glow-pink);
  }

  .search-input::placeholder {
    color: var(--color-text-muted);
  }

  .download-status {
    font-size: 0.75rem;
    color: var(--color-cyan);
    padding: 0 var(--space-md);
    margin-top: calc(-1 * var(--space-xs));
  }
</style>
