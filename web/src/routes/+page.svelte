<script lang="ts">
  import {
    addToQueue,
    fetchDeckStatus,
    fetchNowPlaying,
    fetchQueue,
    fetchSongs,
    pausePlayback,
    removeAllByGuest,
    removeFromQueue,
    resumePlayback,
    searchUSDB,
    triggerDownload,
    USDBNotReadyError,
  } from "$lib/api";
  import type { DeckStatus, USDBResult } from "$lib/api";
  import AppShell from "$lib/components/AppShell.svelte";
  import NameEntry from "$lib/components/NameEntry.svelte";
  import { dedupUSDBResults, libraryKeySet } from "$lib/dedup";
  import { displayElapsed } from "$lib/elapsed";
  import { clearGuestName, getGuestName, setGuestName } from "$lib/stores/session";
  import type { NowPlayingState, QueueEntry, Song } from "$lib/types";
  import BrowseView from "$lib/views/BrowseView.svelte";
  import NowPlayingView from "$lib/views/NowPlayingView.svelte";
  import PartyView from "$lib/views/PartyView.svelte";
  import { onMount } from "svelte";

  const POLL_INTERVAL = 5000;
  // Drives the per-second extrapolation of elapsed time between polls.
  // 250ms is smooth enough for second-resolution display without burning
  // CPU; setInterval (not RAF) keeps ticking when the tab is hidden.
  const TICK_INTERVAL = 250;

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
  let successMessage = $state<string | null>(null);
  let downloadingIds = $state<Set<number>>(new Set());
  let pollTimer = $state<ReturnType<typeof setInterval> | null>(null);
  let tickTimer: ReturnType<typeof setInterval> | null = null;
  // Wall-clock anchor: Date.now() at the moment nowPlaying was last received.
  // Re-set on every successful poll so the projection re-syncs to truth.
  let lastPolledAt = $state(Date.now());
  // Reactive "now" used by the elapsed projection. Updated every TICK_INTERVAL
  // ms, which is what makes the displayed time advance between polls.
  let tickNow = $state(Date.now());
  let searchTimer: ReturnType<typeof setTimeout> | null = null;
  let searchAbort: AbortController | null = null;
  const SEARCH_DEBOUNCE_MS = 300;
  const SEARCH_MIN_CHARS = 2;

  // USDB results minus anything already in the library (normalized match).
  // Keeps "Bohemian Rhapsody (Live Aid)" visible alongside a library
  // "Bohemian Rhapsody" because the titles differ. The library-key set is
  // its own derivation so typing in the search box only re-runs the cheap
  // filter, not the full library normalization.
  const libraryKeys = $derived(libraryKeySet(songs));
  const dedupedUSDB = $derived(dedupUSDBResults(libraryKeys, searchResults));
  // Smoothly-advancing elapsed time. Anchored to the server value at the
  // last poll, projected forward by (tickNow - lastPolledAt). Clamped to
  // duration so it never overshoots; pauses correctly when paused === true.
  const displayedElapsed = $derived(
    nowPlaying === null
      ? 0
      : displayElapsed(
          nowPlaying.elapsed,
          lastPolledAt,
          tickNow,
          nowPlaying.duration,
          paused,
        ),
  );

  function showError(message: string): void {
    errorMessage = message;
    setTimeout(() => { errorMessage = null; }, 4000);
  }

  function showSuccess(message: string): void {
    successMessage = message;
    setTimeout(() => { successMessage = null; }, 2500);
  }

  onMount(() => {
    guestName = getGuestName();

    // Start polling when the app loads.
    startPolling();
    // The tick loop is independent of polling: it can keep advancing the
    // displayed elapsed time even if a poll fails.
    tickTimer = setInterval(() => {
      tickNow = Date.now();
    }, TICK_INTERVAL);

    return () => {
      stopPolling();
      if (tickTimer !== null) {
        clearInterval(tickTimer);
        tickTimer = null;
      }
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
      // Re-anchor the elapsed projection to this server value. Failures
      // intentionally leave the anchor untouched so the local tick keeps
      // advancing through transient network glitches; the next successful
      // poll re-syncs and the duration clamp prevents overshoot if the
      // outage outlasts the song.
      lastPolledAt = Date.now();
    } catch {
      // Polling failures are non-critical.
    }
  }

  function handleJoin(name: string): void {
    setGuestName(name);
    guestName = name;
    // Polling may have been stopped by a previous self-remove ("leave the
    // party"). Restart it idempotently — startPolling is a no-op if already
    // running thanks to the stopPolling call inside it.
    startPolling();
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
    // Optimistic: the entry and the toast appear before the POST resolves,
    // so a tap feels instant on party wifi. The next poll() replaces the
    // provisional entry with server truth; a failed POST removes it.
    // The toast deliberately names no queue position: the server orders
    // round-robin across guests, so the true slot is unknowable here (a
    // first-time guest jumps ahead of a queue-hog's backlog).
    const provisional: QueueEntry = {
      position: queue.length + 1,
      song,
      guest: guestName,
      isNext: queue.length === 0,
    };
    queue = [...queue, provisional];
    showSuccess(
      provisional.isNext
        ? `Added ${song.title} — up next!`
        : `Added ${song.title} to the queue`,
    );
    try {
      await addToQueue(song, guestName);
      await poll();
    } catch {
      queue = queue.filter((entry) => entry !== provisional);
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
        if (err instanceof USDBNotReadyError) {
          showError("USDB warming up — try again in a moment");
        } else {
          showError("Search failed");
        }
      }
    } finally {
      if (searchAbort === controller) {
        searching = false;
      }
    }
  }

  function handleSearchInput(value: string): void {
    searchQuery = value;
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
    // No provisional queue entry here — the song can't be queued until the
    // download lands; the toast acknowledges the tap immediately instead.
    showSuccess(`Added ${result.title} — downloading first`);
    try {
      await triggerDownload(result.id, guestName);
    } catch (err) {
      if (err instanceof USDBNotReadyError) {
        showError("USDB warming up — try again in a moment");
      } else {
        showError("Download failed");
      }
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

  async function handleRemovePerson(name: string, count: number): Promise<void> {
    if (name === guestName) {
      await leaveParty(count);
      return;
    }
    const songWord = count === 1 ? "song" : "songs";
    if (!globalThis.confirm(`Remove all ${String(count)} of ${name}'s ${songWord} from the queue?`)) {
      return;
    }
    try {
      await removeAllByGuest(name);
      await poll();
    } catch {
      showError(`Failed to remove ${name}`);
    }
  }

  async function leaveParty(songCount: number): Promise<void> {
    if (!guestName) return;
    const queuedWord = songCount === 1 ? "song" : "songs";
    const message = songCount > 0
      ? `Leave the party? Your ${String(songCount)} queued ${queuedWord} will be removed.`
      : `Leave the party as ${guestName}?`;
    if (!globalThis.confirm(message)) return;
    try {
      // Always call removeAllByGuest — server is idempotent if zero songs
      // belong to this guest, so callers don't need to branch on count.
      await removeAllByGuest(guestName);
    } catch {
      showError("Failed to leave the party");
      return;
    }
    // Clear local session; stop polling first so the next tick doesn't try
    // to render queue state for a name we just cleared.
    stopPolling();
    clearGuestName();
    guestName = null;
    activeTab = "playing";
  }

  function handleLeavePartyClick(): void {
    const myCount = queue.filter((entry) => entry.guest === guestName).length;
    void leaveParty(myCount);
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
{:else if successMessage}
  <div class="toast toast-success">{successMessage}</div>
{/if}

{#snippet deckOfflineBanner()}
  {#if deckStatus !== null && !deckStatus.online}
    <div class="deck-offline" role="status" aria-live="polite">
      Deck is offline — songs you queue won't play until it's back.
    </div>
  {/if}
{/snippet}

{#snippet identityBadge()}
  <button
    type="button"
    class="identity-badge"
    onclick={handleLeavePartyClick}
    aria-label="Leave the party"
  >
    <span class="identity-name">{guestName}</span>
    <span class="identity-leave" aria-hidden="true">Leave</span>
  </button>
{/snippet}

{#if guestName === null}
  <NameEntry onsubmit={handleJoin} />
{:else}
  <AppShell
    {activeTab}
    onnavigate={handleNavigate}
    banner={deckOfflineBanner}
    headerEnd={identityBadge}
    queueBadge={queue.length}
  >
    {#if activeTab === "playing"}
      <NowPlayingView
        {nowPlaying}
        {displayedElapsed}
        {paused}
        {queue}
        {guestName}
        onpause={() => void handlePause()}
        onresume={() => void handleResume()}
        onremove={(position) => void handleRemove(position)}
        onbrowse={() => handleNavigate("browse")}
      />
    {:else if activeTab === "queue"}
      <PartyView
        {queue}
        {guestName}
        {nowPlaying}
        onremove={(position) => void handleRemove(position)}
        onremoveperson={(name, count) => void handleRemovePerson(name, count)}
        onbrowse={() => handleNavigate("browse")}
      />
    {:else if activeTab === "browse"}
      <BrowseView
        {songs}
        value={searchQuery}
        {searching}
        {loadingSongs}
        {dedupedUSDB}
        {downloadingIds}
        oninput={handleSearchInput}
        onqueue={(song) => void handleQueueSong(song)}
        ondownload={(result) => void handleDownloadAndQueue(result)}
      />
    {/if}
  </AppShell>
{/if}

<style>
  .identity-badge {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 4px 10px;
    background: transparent;
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-full);
    color: var(--color-text);
    font-family: var(--font-body);
    font-size: 0.75rem;
    cursor: pointer;
    transition: color var(--transition-normal), border-color var(--transition-normal),
      box-shadow var(--transition-normal);
  }

  .identity-badge:hover,
  .identity-badge:focus-visible {
    border-color: var(--color-pink);
    box-shadow: var(--glow-pink);
    outline: none;
  }

  .identity-name {
    font-weight: 600;
  }

  .identity-leave {
    color: var(--color-text-muted);
    font-size: 0.6875rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .identity-badge:hover .identity-leave,
  .identity-badge:focus-visible .identity-leave {
    color: var(--color-pink);
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

  .toast-success {
    background: var(--color-green);
    color: var(--color-bg);
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
