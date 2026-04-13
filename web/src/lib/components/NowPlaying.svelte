<script lang="ts">
  let {
    title,
    artist,
    singer,
    elapsed,
    duration,
    paused = false,
    onpause,
    onresume,
  }: {
    title?: string;
    artist?: string;
    singer?: string;
    elapsed?: number;
    duration?: number;
    paused?: boolean;
    onpause?: () => void;
    onresume?: () => void;
  } = $props();

  let isPlaying = $derived(title !== undefined && duration !== undefined);
  let progress = $derived(
    isPlaying && duration && elapsed !== undefined ? (elapsed / duration) * 100 : 0,
  );

  function formatTime(seconds: number): string {
    const m = Math.floor(seconds / 60);
    const s = Math.floor(seconds % 60);
    return `${String(m)}:${String(s).padStart(2, "0")}`;
  }
</script>

<div class="now-playing" class:idle={!isPlaying}>
  {#if isPlaying}
    <div class="header-row">
      <div class="label">{paused ? "PAUSED" : "NOW PLAYING"}</div>
      {#if onpause || onresume}
        <button
          class="playback-toggle"
          type="button"
          onclick={() => paused ? onresume?.() : onpause?.()}
        >
          {paused ? "Resume" : "Pause"}
        </button>
      {/if}
    </div>
    {#if singer}
      <div class="singer">{singer}</div>
    {/if}
    <div class="title">{title}</div>
    <div class="artist">{artist}</div>
    <div class="progress">
      <div class="progress-bar">
        <div class="progress-fill" style="width: {progress}%;"></div>
      </div>
      <div class="progress-times">
        <span>{formatTime(elapsed ?? 0)}</span>
        <span>{formatTime(duration ?? 0)}</span>
      </div>
    </div>
  {:else}
    <div class="idle-content">
      <span class="idle-icon">&#9835;</span>
      <span class="idle-text">No song playing</span>
    </div>
  {/if}
</div>

<style>
  .now-playing {
    padding: var(--space-lg);
    background: var(--color-surface);
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .now-playing.idle {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 160px;
  }

  .header-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: var(--space-sm);
  }

  .label {
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    color: var(--color-pink);
    text-shadow: var(--glow-text-pink);
  }

  .playback-toggle {
    font-family: var(--font-body);
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--color-text-muted);
    background: none;
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-sm);
    padding: 4px 12px;
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .playback-toggle:hover {
    color: var(--color-text);
    border-color: var(--color-pink);
  }

  .singer {
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--color-cyan);
    margin-bottom: var(--space-xs);
  }

  .title {
    font-size: 1.5rem;
    font-weight: 700;
    color: var(--color-text);
    line-height: 1.2;
    margin-bottom: 2px;
  }

  .artist {
    font-size: 1rem;
    color: var(--color-text-dim);
    margin-bottom: var(--space-md);
  }

  .progress {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .progress-bar {
    width: 100%;
    height: 3px;
    background: var(--color-surface-raised);
    border-radius: var(--radius-full);
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: var(--color-pink);
    border-radius: var(--radius-full);
    box-shadow: 0 0 6px rgba(255, 45, 123, 0.5);
    transition: width 1s linear;
  }

  .progress-times {
    display: flex;
    justify-content: space-between;
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }

  .idle-content {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-sm);
  }

  .idle-icon {
    font-size: 2rem;
    color: var(--color-text-muted);
    opacity: 0.5;
  }

  .idle-text {
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }
</style>
