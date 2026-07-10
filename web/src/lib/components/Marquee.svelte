<script lang="ts">
  import type { Snippet } from "svelte";

  let { children }: { children: Snippet } = $props();

  const PIXELS_PER_SECOND = 40;
  const GAP_PX = 48;

  let containerEl: HTMLDivElement | undefined = $state();
  let measureEl: HTMLSpanElement | undefined = $state();
  let scrolling = $state(false);
  let duration = $state(0);

  $effect(() => {
    if (!containerEl || !measureEl) return;
    const overflowing = measureEl.scrollWidth > containerEl.clientWidth;
    scrolling = overflowing;
    duration = overflowing ? (measureEl.scrollWidth + GAP_PX) / PIXELS_PER_SECOND : 0;
  });
</script>

<div class="marquee" bind:this={containerEl}>
  <div
    class="track"
    class:scrolling
    style={scrolling
      ? `--marquee-duration: ${String(duration)}s; --marquee-gap: ${String(GAP_PX)}px;`
      : undefined}
  >
    <span class="segment">
      <span class="measure" bind:this={measureEl}>{@render children()}</span>
    </span>
    {#if scrolling}
      <span class="segment" aria-hidden="true">
        <span class="measure">{@render children()}</span>
      </span>
    {/if}
  </div>
</div>

<style>
  .marquee {
    overflow: hidden;
    white-space: nowrap;
    width: 100%;
  }

  .track {
    display: inline-flex;
    width: max-content;
  }

  .segment {
    display: inline-block;
    flex-shrink: 0;
    padding-right: var(--marquee-gap, 0px);
  }

  .track:not(.scrolling) .segment {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    padding-right: 0;
  }

  .track:not(.scrolling) .measure {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  @media (prefers-reduced-motion: no-preference) {
    .track.scrolling {
      animation: marquee-scroll var(--marquee-duration) linear 2s infinite;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .track {
      width: 100%;
    }

    .track .segment:nth-child(2) {
      display: none;
    }

    .track .segment {
      max-width: 100%;
      overflow: hidden;
      text-overflow: ellipsis;
      padding-right: 0;
    }

    .track .measure {
      display: block;
      overflow: hidden;
      text-overflow: ellipsis;
    }
  }

  @keyframes marquee-scroll {
    from {
      transform: translateX(0);
    }
    to {
      transform: translateX(-50%);
    }
  }
</style>
