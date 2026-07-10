<script lang="ts">
  import Button from "$lib/components/Button.svelte";
  import GuestChip from "$lib/components/GuestChip.svelte";
  import QueueItem from "$lib/components/QueueItem.svelte";
  import { celebrationFor, waitEstimate } from "$lib/party";
  import type { NowPlayingState, QueueEntry } from "$lib/types";
  import { untrack } from "svelte";
  import { SvelteMap } from "svelte/reactivity";

  let {
    queue,
    guestName,
    nowPlaying = null,
    onremove,
    onremoveperson,
    onbrowse,
  }: {
    queue: QueueEntry[];
    guestName: string;
    nowPlaying?: NowPlayingState | null;
    onremove: (position: number) => void;
    onremoveperson: (name: string, count: number) => void;
    onbrowse: () => void;
  } = $props();

  const CELEBRATION_MS = 4000;

  // Derive ordered (guest, count) pairs from the queue. Insertion-order is
  // preserved by Map, which mirrors the round-robin guestOrder on the server.
  const partyPeople = $derived.by(() => {
    const counts = new SvelteMap<string, number>();
    for (const entry of queue) {
      counts.set(entry.guest, (counts.get(entry.guest) ?? 0) + 1);
    }
    return Array.from(counts, ([name, count]) => ({ name, count }));
  });

  const isMineNext = $derived(queue[0]?.guest === guestName);

  const remaining = $derived(
    nowPlaying
      ? Math.max(0, nowPlaying.duration - nowPlaying.elapsed)
      : undefined,
  );

  // Tracks the previous queue snapshot so celebrationFor can detect a
  // guest's song transitioning from "queued" to "now playing" between
  // renders. Read via untrack so writing it back doesn't re-trigger this
  // same effect.
  let lastQueue = $state<QueueEntry[]>([]);
  let celebrating = $state(false);
  let celebrationTimeout: ReturnType<typeof setTimeout> | undefined;

  $effect(() => {
    const currentQueue = queue;
    const previousQueue = untrack(() => lastQueue);
    if (
      celebrationFor(previousQueue, currentQueue, nowPlaying?.id, guestName)
    ) {
      celebrating = true;
      clearTimeout(celebrationTimeout);
      celebrationTimeout = setTimeout(() => {
        celebrating = false;
      }, CELEBRATION_MS);
    }
    lastQueue = currentQueue;
  });

  $effect(() => {
    return () => clearTimeout(celebrationTimeout);
  });
</script>

<div class="section">
  {#if partyPeople.length > 0}
    <div class="section-head">
      <div class="section-label">
        {partyPeople.length}
        {partyPeople.length === 1 ? "friend" : "friends"} ·
        {queue.length}
        {queue.length === 1 ? "song" : "songs"}
      </div>
    </div>
    <div class="people-chips">
      {#each partyPeople as person (person.name)}
        <button
          type="button"
          class="people-chip"
          class:me={person.name === guestName}
          onclick={() => onremoveperson(person.name, person.count)}
          aria-label="Remove all of {person.name}'s songs"
        >
          <GuestChip name={person.name} />
          <span class="chip-name">{person.name}</span>
          <span class="chip-count">{person.count}</span>
          <span class="chip-x" aria-hidden="true">&times;</span>
        </button>
      {/each}
    </div>
  {/if}

  {#if queue.length > 0}
    <div class="section-head" style="margin-top: var(--space-md);">
      <div class="section-label">Up next</div>
      <div class="section-sub">Round-robin order</div>
    </div>
    {#if isMineNext}
      <div class="up-next-banner">You're up next, {guestName}! 🎤</div>
    {/if}
    <div class="list">
      {#each queue as entry (entry.position)}
        <QueueItem
          position={entry.position}
          title={entry.song.title}
          artist={entry.song.artist}
          guest={entry.guest}
          isNext={entry.isNext}
          waitText={waitEstimate(entry.position, remaining)}
          onremove={entry.guest === guestName
            ? () => onremove(entry.position)
            : undefined}
        />
      {/each}
    </div>
  {:else}
    <div class="empty-prompt">
      <p>No songs queued yet.</p>
      <Button onclick={onbrowse}>Browse Songs</Button>
    </div>
  {/if}
</div>

{#if celebrating}
  <div class="celebration">
    <p>You're on, {guestName}! 🎤</p>
  </div>
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

  .people-chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-xs);
    margin-bottom: var(--space-md);
  }

  .people-chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 8px 6px 8px;
    background: var(--color-surface);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-full);
    color: var(--color-text);
    font-family: var(--font-body);
    font-size: 0.8125rem;
    cursor: pointer;
    transition:
      color var(--transition-normal),
      border-color var(--transition-normal),
      box-shadow var(--transition-normal);
  }

  .people-chip:hover,
  .people-chip:focus-visible {
    border-color: var(--color-pink);
    color: var(--color-text);
    box-shadow: var(--glow-pink);
    outline: none;
  }

  .people-chip.me {
    border-color: var(--color-pink);
    color: var(--color-pink);
  }

  .chip-name {
    font-weight: 600;
  }

  .chip-count {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    background: var(--color-surface-raised);
    padding: 1px 6px;
    border-radius: var(--radius-full);
    line-height: 1.4;
  }

  .chip-x {
    font-size: 1.1rem;
    line-height: 1;
    color: var(--color-text-muted);
    margin-left: 2px;
  }

  .people-chip:hover .chip-x,
  .people-chip:focus-visible .chip-x {
    color: var(--color-pink);
  }

  .up-next-banner {
    margin-bottom: var(--space-md);
    padding: var(--space-sm) var(--space-md);
    background: var(--color-surface);
    border: 1px solid var(--color-pink);
    border-radius: var(--radius-md);
    box-shadow: var(--glow-pink);
    color: var(--color-pink);
    font-weight: 600;
    font-size: 0.875rem;
    text-align: center;
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

  .celebration {
    position: fixed;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(10, 10, 15, 0.92);
    z-index: 50;
    animation: celebration-pop 0.4s ease-out;
  }

  .celebration p {
    font-size: 1.5rem;
    font-weight: 700;
    color: var(--color-pink);
    text-shadow: var(--glow-text-pink);
  }

  @keyframes celebration-pop {
    from {
      opacity: 0;
      transform: scale(0.85);
    }
    to {
      opacity: 1;
      transform: scale(1);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .celebration {
      animation: none;
    }
  }
</style>
