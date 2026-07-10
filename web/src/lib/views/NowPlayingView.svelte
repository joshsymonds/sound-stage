<script lang="ts">
  import Button from "$lib/components/Button.svelte";
  import NowPlaying from "$lib/components/NowPlaying.svelte";
  import QueueItem from "$lib/components/QueueItem.svelte";
  import type { NowPlayingState, QueueEntry } from "$lib/types";

  let {
    nowPlaying,
    displayedElapsed,
    paused,
    queue,
    guestName,
    onpause,
    onresume,
    onremove,
    onbrowse,
  }: {
    nowPlaying: NowPlayingState | null;
    displayedElapsed: number;
    paused: boolean;
    queue: QueueEntry[];
    guestName: string;
    onpause: () => void;
    onresume: () => void;
    onremove: (position: number) => void;
    onbrowse: () => void;
  } = $props();
</script>

<NowPlaying
  title={nowPlaying?.title}
  artist={nowPlaying?.artist}
  elapsed={nowPlaying === null ? undefined : displayedElapsed}
  duration={nowPlaying?.duration}
  {paused}
  {onpause}
  {onresume}
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
          onremove={entry.guest === guestName ? () => onremove(entry.position) : undefined}
        />
      {/each}
    </div>
  </div>
{:else}
  <div class="empty-prompt">
    <p>Queue a song to get started, {guestName}!</p>
    <Button onclick={onbrowse}>Browse Songs</Button>
  </div>
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
</style>
