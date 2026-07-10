<script lang="ts">
  import Button from "$lib/components/Button.svelte";
  import QueueItem from "$lib/components/QueueItem.svelte";
  import type { QueueEntry } from "$lib/types";
  import { SvelteMap } from "svelte/reactivity";

  let {
    queue,
    guestName,
    onremove,
    onremoveperson,
    onbrowse,
  }: {
    queue: QueueEntry[];
    guestName: string;
    onremove: (position: number) => void;
    onremoveperson: (name: string, count: number) => void;
    onbrowse: () => void;
  } = $props();

  // Derive ordered (guest, count) pairs from the queue. Insertion-order is
  // preserved by Map, which mirrors the round-robin guestOrder on the server.
  const partyPeople = $derived.by(() => {
    const counts = new SvelteMap<string, number>();
    for (const entry of queue) {
      counts.set(entry.guest, (counts.get(entry.guest) ?? 0) + 1);
    }
    return Array.from(counts, ([name, count]) => ({ name, count }));
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
    <div class="list">
      {#each queue as entry (entry.position)}
        <QueueItem
          position={entry.position}
          title={entry.song.title}
          artist={entry.song.artist}
          guest={entry.guest}
          isNext={entry.isNext}
          onremove={entry.guest === guestName ? () => onremove(entry.position) : undefined}
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
    padding: 6px 8px 6px 12px;
    background: var(--color-surface);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-full);
    color: var(--color-text);
    font-family: var(--font-body);
    font-size: 0.8125rem;
    cursor: pointer;
    transition: color var(--transition-normal), border-color var(--transition-normal),
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
</style>
