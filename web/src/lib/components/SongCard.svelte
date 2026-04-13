<script lang="ts">
  let {
    title,
    artist,
    edition,
    year,
    coverUrl,
    onclick,
  }: {
    title: string;
    artist: string;
    edition?: string;
    year?: number;
    coverUrl?: string;
    onclick?: () => void;
  } = $props();

  let interactive = $derived(onclick !== undefined);
</script>

<button
  class="song-card"
  class:interactive
  type="button"
  onclick={onclick}
  disabled={!interactive}
>
  <div class="cover">
    {#if coverUrl}
      <img src={coverUrl} alt="{artist} — {title}" class="cover-img" />
    {:else}
      <div class="cover-placeholder">
        <span class="cover-icon">&#9835;</span>
      </div>
    {/if}
  </div>
  <div class="info">
    <span class="title">{title}</span>
    <span class="artist">{artist}</span>
    {#if edition || year}
      <span class="meta">
        {#if edition}{edition}{/if}
        {#if edition && year} &middot; {/if}
        {#if year}{year}{/if}
      </span>
    {/if}
  </div>
</button>

<style>
  .song-card {
    display: flex;
    align-items: center;
    gap: var(--space-md);
    padding: var(--space-sm) var(--space-md);
    background: var(--color-surface);
    border-radius: var(--radius-md);
    border: 1px solid var(--color-border-subtle);
    width: 100%;
    text-align: left;
    font-family: var(--font-body);
    color: var(--color-text);
    cursor: default;
    transition: all var(--transition-normal);
  }

  .song-card.interactive {
    cursor: pointer;
  }

  .song-card.interactive:hover {
    border-color: var(--color-pink);
    box-shadow: var(--glow-pink);
    background: var(--color-surface-raised);
  }

  .cover {
    flex-shrink: 0;
    width: 48px;
    height: 48px;
    border-radius: var(--radius-sm);
    overflow: hidden;
  }

  .cover-img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .cover-placeholder {
    width: 100%;
    height: 100%;
    background: var(--color-surface-raised);
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .cover-icon {
    font-size: 1.25rem;
    color: var(--color-text-muted);
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
    font-size: 0.8125rem;
    color: var(--color-text-dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .meta {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
  }
</style>
