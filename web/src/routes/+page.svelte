<script lang="ts">
  import {
    addToQueue,
    fetchNowPlaying,
    fetchQueue,
    fetchSongs,
    pausePlayback,
    removeFromQueue,
    resumePlayback,
    searchUSDB,
    triggerDownload,
  } from "$lib/api";
  import type { USDBResult } from "$lib/api";
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
  let searchResults = $state<USDBResult[]>([]);
  let searchQuery = $state("");
  let loadingSongs = $state(false);
  let searching = $state(false);
  let paused = $state(false);
  let errorMessage = $state<string | null>(null);
  let downloadingIds = $state<Set<number>>(new Set());
  let pollTimer = $state<ReturnType<typeof setInterval> | null>(null);

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
      const [queueData, nowPlayingData] = await Promise.all([
        fetchQueue(),
        fetchNowPlaying(),
      ]);
      queue = queueData;
      nowPlaying = nowPlayingData;
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

  async function handleSearch(): Promise<void> {
    const query = searchQuery.trim();
    if (query === "" || searching) return;
    searching = true;
    searchResults = [];
    try {
      searchResults = await searchUSDB({ title: query });
    } catch {
      searchResults = [];
      showError("Search failed");
    } finally {
      searching = false;
    }
  }

  function handleSearchKeydown(event: KeyboardEvent): void {
    if (event.key === "Enter") {
      void handleSearch();
    }
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

{#if guestName === null}
  <NameEntry onsubmit={handleJoin} />
{:else}
  <AppShell {activeTab} onnavigate={handleNavigate}>
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
            type="text"
            class="search-input"
            placeholder="Search USDB for songs..."
            bind:value={searchQuery}
            onkeydown={handleSearchKeydown}
          />
          <Button size="sm" onclick={() => void handleSearch()} disabled={searching}>
            {searching ? "..." : "Search"}
          </Button>
        </div>

        {#if searchResults.length > 0}
          <div class="section-label" style="margin-top: var(--space-md);">USDB RESULTS</div>
          <div class="list">
            {#each searchResults as result (result.id)}
              <SongCard
                title={result.title}
                artist={result.artist}
                onclick={() => void handleDownloadAndQueue(result)}
              />
              {#if downloadingIds.has(result.id)}
                <div class="download-status">Downloading...</div>
              {/if}
            {/each}
          </div>
        {/if}

        <div class="section-label" style="margin-top: var(--space-md);">
          {#if loadingSongs}LOADING...{:else}LIBRARY{/if}
        </div>
        {#if songs.length > 0}
          <div class="list">
            {#each songs as song (song.id)}
              <SongCard
                title={song.title}
                artist={song.artist}
                edition={song.edition}
                year={song.year}
                onclick={() => void handleQueueSong(song)}
              />
            {/each}
          </div>
        {:else if !loadingSongs}
          <div class="empty-prompt">
            <p>No songs in the library. Search USDB above to download some!</p>
          </div>
        {/if}
      </div>
    {/if}
  </AppShell>
{/if}

<style>
  .section {
    padding: var(--space-md) var(--space-lg);
  }

  .section-label {
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    color: var(--color-pink);
    text-shadow: var(--glow-text-pink);
    margin-bottom: var(--space-sm);
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
</style>
