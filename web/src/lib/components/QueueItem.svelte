<script lang="ts">
  let {
    position,
    title,
    artist,
    guest,
    isNext = false,
    onremove,
  }: {
    position: number;
    title: string;
    artist: string;
    guest: string;
    isNext?: boolean;
    onremove?: () => void;
  } = $props();
</script>

<div class="queue-item" class:next={isNext}>
  <span class="position" class:next={isNext}>{position}</span>
  <div class="info">
    <span class="title">{title}</span>
    <span class="artist">{artist}</span>
  </div>
  <span class="guest">{guest}</span>
  {#if onremove}
    <button type="button" class="remove" aria-label="Remove your song" onclick={onremove}>
      <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
        <path
          d="M3 4h10M6.5 4V2.5h3V4M5 4l.6 9a1 1 0 0 0 1 .9h2.8a1 1 0 0 0 1-.9L11 4"
          fill="none"
          stroke="currentColor"
          stroke-width="1.4"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
    </button>
  {/if}
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

  .remove {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    margin-left: var(--space-xs);
    padding: 0;
    background: transparent;
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    cursor: pointer;
    flex-shrink: 0;
    transition: color var(--transition-normal), border-color var(--transition-normal),
      box-shadow var(--transition-normal);
  }

  .remove:hover,
  .remove:focus-visible {
    color: var(--color-pink);
    border-color: var(--color-pink);
    box-shadow: var(--glow-pink);
    outline: none;
  }
</style>
