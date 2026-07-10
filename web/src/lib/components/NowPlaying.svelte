<script lang="ts">
  import { dominantColor } from "$lib/color";
  import { untrack } from "svelte";

  import EqualizerGlyph from "./EqualizerGlyph.svelte";
  import Marquee from "./Marquee.svelte";

  let {
    id,
    title,
    artist,
    singer,
    elapsed,
    duration,
    paused = false,
    onpause,
    onresume,
  }: {
    id?: string;
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
  let backdropFailed = $state(false);
  let lastBackdropId = $state(untrack(() => id));
  let glowColor = $state<string | null>(null);

  $effect(() => {
    if (id !== lastBackdropId) {
      lastBackdropId = id;
      backdropFailed = false;
    }
  });

  $effect(() => {
    const heroId = id;
    if (!isPlaying || !heroId) {
      glowColor = null;
      return;
    }
    void dominantColor(`/api/library/${heroId}/thumb`).then((color) => {
      if (heroId === id) {
        glowColor = color;
      }
    });
  });

  function formatTime(seconds: number): string {
    const m = Math.floor(seconds / 60);
    const s = Math.floor(seconds % 60);
    return `${String(m)}:${String(s).padStart(2, "0")}`;
  }
</script>

<div
  class="now-playing"
  class:idle={!isPlaying}
  style={glowColor ? `--hero-glow: ${glowColor}` : undefined}
>
  {#if isPlaying}
    {#if id && !backdropFailed}
      <img
        class="backdrop"
        aria-hidden="true"
        alt=""
        src={`/api/library/${id}/cover`}
        onerror={() => {
          backdropFailed = true;
        }}
      />
      <div class="scrim"></div>
    {/if}
    <div class="content">
      <div class="header-row">
        <div class="eyebrow">
          <EqualizerGlyph playing={!paused} />
          <span class="label">{paused ? "PAUSED" : "NOW PLAYING"}</span>
        </div>
        {#if onpause || onresume}
          <button
            class="playback-toggle"
            type="button"
            onclick={() => (paused ? onresume?.() : onpause?.())}
          >
            {paused ? "Resume" : "Pause"}
          </button>
        {/if}
      </div>
      <div class="title">
        {#key title}
          <Marquee>{title}</Marquee>
        {/key}
      </div>
      <div class="artist">
        {#key artist}
          <Marquee>{artist}</Marquee>
        {/key}
      </div>
      {#if singer}
        <div class="singer">🎤 {singer}</div>
      {/if}
      <div class="progress">
        <div class="progress-bar">
          <div class="progress-fill" style="width: {progress}%;"></div>
        </div>
        <div class="progress-times">
          <span>{formatTime(elapsed ?? 0)}</span>
          <span>{formatTime(duration ?? 0)}</span>
        </div>
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
    position: relative;
    overflow: hidden;
    min-height: 340px;
    display: flex;
    background: var(--color-surface);
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .now-playing.idle {
    align-items: center;
    justify-content: center;
    min-height: 160px;
  }

  .backdrop {
    position: absolute;
    inset: 0;
    z-index: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
    filter: blur(24px) saturate(1.4) brightness(0.55);
    transform: scale(1.2);
  }

  .scrim {
    position: absolute;
    inset: 0;
    z-index: 0;
    background:
      radial-gradient(
        ellipse at 50% 20%,
        color-mix(in srgb, var(--hero-glow, var(--color-pink)) 25%, transparent),
        transparent 70%
      ),
      linear-gradient(to top, rgba(10, 10, 15, 0.85), transparent);
  }

  .content {
    position: relative;
    z-index: 1;
    width: 100%;
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
    padding: var(--space-lg);
  }

  .header-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: var(--space-sm);
  }

  .eyebrow {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
  }

  .label {
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    color: var(--hero-glow, var(--color-pink));
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

  .title {
    font-size: 2.5rem;
    font-weight: 800;
    color: var(--color-text);
    line-height: 1.15;
    margin-bottom: 2px;
  }

  .artist {
    font-size: 1.5rem;
    color: var(--color-text-dim);
    margin-bottom: var(--space-sm);
  }

  .singer {
    font-size: 1.25rem;
    font-weight: 600;
    color: var(--color-pink);
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
    background: rgba(255, 255, 255, 0.15);
    border-radius: var(--radius-full);
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: var(--hero-glow, var(--color-pink));
    border-radius: var(--radius-full);
    box-shadow: 0 0 6px rgba(255, 45, 123, 0.5);
    transition: width 1s linear;
  }

  .progress-times {
    display: flex;
    justify-content: space-between;
    font-size: 0.75rem;
    color: var(--color-text-dim);
    text-shadow: 0 1px 3px rgba(0, 0, 0, 0.6);
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
