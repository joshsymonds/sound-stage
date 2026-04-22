<script lang="ts">
  import {
    addToQueue,
    fetchDeckStatus,
    fetchNowPlaying,
    fetchQueue,
    fetchSongs,
    pausePlayback,
    removeFromQueue,
    resumePlayback,
    searchUSDB,
    triggerDownload,
  } from "$lib/api";
  import type { DeckStatus, USDBResult } from "$lib/api";
  import AppShell from "$lib/components/AppShell.svelte";
  import Button from "$lib/components/Button.svelte";
  import NameEntry from "$lib/components/NameEntry.svelte";
  import NowPlaying from "$lib/components/NowPlaying.svelte";
  import QueueItem from "$lib/components/QueueItem.svelte";
  import SongCard from "$lib/components/SongCard.svelte";
  import { getGuestName, setGuestName } from "$lib/stores/session";
  import type { NowPlayingState, QueueEntry, Song } from "$lib/types";
  import { onMount } from "svelte";

  const POLL_INTERVAL = 5000;

  let guestName = $state<string | null>(null);
  let activeTab = $state<"playing" | "queue" | "browse">("playing");
  let songs = $state<Song[]>([]);
  let queue = $state<QueueEntry[]>([]);
  let nowPlaying = $state<NowPlayingState | null>(null);
  let deckStatus = $state<DeckStatus | null>(null);
  let searchResults = $state<USDBResult[]>([]);
  let searchQuery = $state("");
  let loadingSongs = $state(false);
  let searching = $state(false);
  let paused = $state(false);
  let errorMessage = $state<string | null>(null);
  let downloadingIds = $state<Set<number>>(new Set());
  let pollTimer = $state<ReturnType<typeof setInterval> | null>(null);
  let searchTimer: ReturnType<typeof setTimeout> | null = null;
  let searchAbort: AbortController | null = null;
  const SEARCH_DEBOUNCE_MS = 300;
  const SEARCH_MIN_CHARS = 2;

  // Library filter is client-side and instant — no debounce. Songs is at most
  // a few thousand entries; substring match across title + artist is cheap.
  const filteredSongs = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    if (q.length < SEARCH_MIN_CHARS) return songs;
    return songs.filter(
      (s) => s.title.toLowerCase().includes(q) || s.artist.toLowerCase().includes(q),
    );
  });
  const isSearching = $derived(searchQuery.trim().length >= SEARCH_MIN_CHARS);

  function showError(message: string): void {
    errorMessage = message;
    setTimeout(() => { errorMessage = null; }, 4000);
  }

  onMount(() => {
    guestName = getGuestName();

    // Start polling when the app loads.
    startPolling();

    return () => {
      stopPolling();
    };
  });

  function startPolling(): void {
    stopPolling();
    void poll();
    pollTimer = setInterval(() => void poll(), POLL_INTERVAL);
  }

  function stopPolling(): void {
    if (pollTimer !== null) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  async function poll(): Promise<void> {
    try {
      const [queueData, nowPlayingData, deckStatusData] = await Promise.all([
        fetchQueue(),
        fetchNowPlaying(),
        fetchDeckStatus(),
      ]);
      queue = queueData;
      nowPlaying = nowPlayingData;
      deckStatus = deckStatusData;
    } catch {
      // Polling failures are non-critical.
    }
  }

  function handleJoin(name: string): void {
    setGuestName(name);
    guestName = name;
  }

  async function loadSongs(): Promise<void> {
    if (loadingSongs) return;
    loadingSongs = true;
    try {
      songs = await fetchSongs();
    } catch {
      songs = [];
      showError("Failed to load songs");
    } finally {
      loadingSongs = false;
    }
  }

  async function handleQueueSong(song: Song): Promise<void> {
    if (!guestName) return;
    try {
      await addToQueue(song, guestName);
      await poll();
      activeTab = "queue";
    } catch {
      showError("Failed to queue song");
    }
  }

  async function runSearch(query: string): Promise<void> {
    // Cancel any in-flight request before starting a new one — keeps stale
    // results from overwriting fresh ones if the network reorders them.
    searchAbort?.abort();
    const controller = new AbortController();
    searchAbort = controller;

    searching = true;
    try {
      const results = await searchUSDB({ title: query }, controller.signal);
      // Only commit if we're still the latest in-flight call.
      if (searchAbort === controller) {
        searchResults = results;
      }
    } catch (err) {
      if ((err as { name?: string }).name === "AbortError") return;
      if (searchAbort === controller) {
        searchResults = [];
        showError("Search failed");
      }
    } finally {
      if (searchAbort === controller) {
        searching = false;
      }
    }
  }

  function handleSearchInput(): void {
    if (searchTimer !== null) clearTimeout(searchTimer);
    const query = searchQuery.trim();
    if (query.length < SEARCH_MIN_CHARS) {
      // Clear results when the input gets too short so guests don't see
      // stale matches from a longer query they just deleted.
      searchAbort?.abort();
      searchResults = [];
      searching = false;
      return;
    }
    searchTimer = setTimeout(() => void runSearch(query), SEARCH_DEBOUNCE_MS);
  }

  async function handleDownloadAndQueue(result: USDBResult): Promise<void> {
    if (!guestName) return;
    if (downloadingIds.has(result.id)) return;
    downloadingIds = new Set([...downloadingIds, result.id]);
    try {
      await triggerDownload(result.id, guestName);
      activeTab = "queue";
    } catch {
      showError("Download failed");
    }
  }

  async function handleRemove(position: number): Promise<void> {
    if (!guestName) return;
    try {
      await removeFromQueue(position, guestName);
      await poll();
    } catch {
      showError("Failed to remove song");
    }
  }

  async function handlePause(): Promise<void> {
    await pausePlayback();
    paused = true;
  }

  async function handleResume(): Promise<void> {
    await resumePlayback();
    paused = false;
  }

  function handleNavigate(tab: string): void {
    activeTab = tab as "playing" | "queue" | "browse";
    if (tab === "browse" && songs.length === 0) {
      void loadSongs();
    }
  }
</script>

<svelte:head>
  <title>SoundStage</title>
</svelte:head>

{#if errorMessage}
  <div class="toast">{errorMessage}</div>
{/if}

{#snippet deckOfflineBanner()}
  {#if deckStatus !== null && !deckStatus.online}
    <div class="deck-offline" role="status" aria-live="polite">
      Deck is offline — songs you queue won't play until it's back.
    </div>
  {/if}
{/snippet}

{#if guestName === null}
  <NameEntry onsubmit={handleJoin} />
{:else}
  <AppShell {activeTab} onnavigate={handleNavigate} banner={deckOfflineBanner}>
    {#if activeTab === "playing"}
      <NowPlaying
        title={nowPlaying?.title}
        artist={nowPlaying?.artist}
        elapsed={nowPlaying?.elapsed}
        duration={nowPlaying?.duration}
        {paused}
        onpause={() => void handlePause()}
        onresume={() => void handleResume()}
      />
      {#if queue.length > 0}
        <div class="section">
          <div class="section-label">UP NEXT</div>
          <div class="list">
            {#each queue.slice(0, 3) as entry (entry.position)}
              <QueueItem
                position={entry.position}
                title={entry.song.title}
                artist={entry.song.artist}
                guest={entry.guest}
                isNext={entry.isNext}
                onremove={entry.guest === guestName
                  ? () => void handleRemove(entry.position)
                  : undefined}
              />
            {/each}
          </div>
        </div>
      {:else}
        <div class="empty-prompt">
          <p>Queue a song to get started, {guestName}!</p>
          <Button onclick={() => handleNavigate("browse")}>Browse Songs</Button>
        </div>
      {/if}

    {:else if activeTab === "queue"}
      <div class="section">
        <div class="section-label">QUEUE</div>
        {#if queue.length > 0}
          <div class="list">
            {#each queue as entry (entry.position)}
              <QueueItem
                position={entry.position}
                title={entry.song.title}
                artist={entry.song.artist}
                guest={entry.guest}
                isNext={entry.isNext}
                onremove={entry.guest === guestName
                  ? () => void handleRemove(entry.position)
                  : undefined}
              />
            {/each}
          </div>
        {:else}
          <div class="empty-prompt">
            <p>No songs in the queue yet.</p>
            <Button onclick={() => handleNavigate("browse")}>Browse Songs</Button>
          </div>
        {/if}
      </div>

    {:else if activeTab === "browse"}
      <div class="section">
        <div class="search-bar">
          <input
            type="search"
            class="search-input"
            placeholder="Search by title or artist…"
            bind:value={searchQuery}
            oninput={handleSearchInput}
          />
          {#if searching}
            <span class="search-spinner" aria-label="Searching">…</span>
          {/if}
        </div>

        <div class="section-head" style="margin-top: var(--space-md);">
          <div class="section-label">
            {isSearching ? "Library matches" : "In your library"}
          </div>
          <div class="section-sub">Plays instantly</div>
        </div>
        {#if loadingSongs}
          <div class="empty-prompt"><p>Loading…</p></div>
        {:else if filteredSongs.length > 0}
          <div class="list">
            {#each filteredSongs as song (song.id)}
              <SongCard
                title={song.title}
                artist={song.artist}
                edition={song.edition}
                year={song.year}
                coverUrl={"/api/library/" + song.id + "/cover"}
                onclick={() => void handleQueueSong(song)}
              />
            {/each}
          </div>
        {:else if isSearching}
          <div class="empty-prompt"><p>No library matches for &ldquo;{searchQuery}&rdquo;.</p></div>
        {:else}
          <div class="empty-prompt">
            <p>Nothing downloaded yet. Search above to grab a song.</p>
          </div>
        {/if}

        {#if isSearching}
          <div class="section-head" style="margin-top: var(--space-lg);">
            <div class="section-label">From the USDB catalog</div>
            <div class="section-sub">Tap to download (~30s) and queue</div>
          </div>
          {#if searching && searchResults.length === 0}
            <div class="empty-prompt"><p>Searching USDB…</p></div>
          {:else if searchResults.length > 0}
            <div class="list">
              {#each searchResults as result (result.id)}
                <SongCard
                  title={result.title}
                  artist={result.artist}
                  coverUrl={"/api/usdb/cover/" + String(result.id)}
                  onclick={() => void handleDownloadAndQueue(result)}
                />
                {#if downloadingIds.has(result.id)}
                  <div class="download-status">Downloading…</div>
                {/if}
              {/each}
            </div>
          {:else}
            <div class="empty-prompt"><p>No USDB matches.</p></div>
          {/if}
        {/if}
      </div>
    {/if}
  </AppShell>
{/if}

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

  .toast {
    position: fixed;
    top: var(--space-md);
    left: 50%;
    transform: translateX(-50%);
    background: var(--color-red);
    color: white;
    padding: var(--space-sm) var(--space-lg);
    border-radius: var(--radius-md);
    font-size: 0.875rem;
    font-weight: 500;
    z-index: 100;
    animation: fade-slide-up 200ms ease;
  }

  .deck-offline {
    flex-shrink: 0;
    padding: var(--space-xs) var(--space-md);
    background: rgba(255, 70, 70, 0.18);
    border-bottom: 1px solid var(--color-red);
    color: var(--color-text);
    font-size: 0.75rem;
    font-weight: 500;
    text-align: center;
  }
</style>
