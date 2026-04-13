<script lang="ts">
  let {
    position,
    title,
    artist,
    guest,
    isNext = false,
  }: {
    position: number;
    title: string;
    artist: string;
    guest: string;
    isNext?: boolean;
  } = $props();
</script>

<div class="queue-item" class:next={isNext}>
  <span class="position" class:next={isNext}>{position}</span>
  <div class="info">
    <span class="title">{title}</span>
    <span class="artist">{artist}</span>
  </div>
  <span class="guest">{guest}</span>
</div>

<style>
  .queue-item {
    display: flex;
    align-items: center;
    gap: var(--space-md);
    padding: var(--space-sm) var(--space-md);
    background: var(--color-surface);
    border-radius: var(--radius-md);
    border: 1px solid var(--color-border-subtle);
    transition: all var(--transition-normal);
  }

  .queue-item.next {
    border-color: var(--color-pink);
    box-shadow: var(--glow-pink);
  }

  .position {
    font-size: 0.875rem;
    font-weight: 700;
    color: var(--color-text-muted);
    width: 24px;
    text-align: center;
    flex-shrink: 0;
  }

  .position.next {
    color: var(--color-pink);
  }

  .info {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .title {
    font-size: 0.875rem;
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

  .guest {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    flex-shrink: 0;
  }
</style>
