<script lang="ts">
  let { playing = false }: { playing?: boolean } = $props();
</script>

<div class="equalizer" class:playing aria-hidden="true">
  <span class="bar"></span>
  <span class="bar"></span>
  <span class="bar"></span>
</div>

<style>
  .equalizer {
    display: inline-flex;
    align-items: flex-end;
    gap: 3px;
    height: 16px;
    flex-shrink: 0;
  }

  .bar {
    width: 4px;
    height: 4px;
    background: var(--color-pink);
    border-radius: 2px;
    box-shadow: var(--glow-pink);
  }

  .equalizer.playing .bar {
    animation: eq-bounce 0.9s ease-in-out infinite;
  }

  .equalizer.playing .bar:nth-child(1) {
    animation-delay: 0s;
  }

  .equalizer.playing .bar:nth-child(2) {
    animation-delay: 0.2s;
  }

  .equalizer.playing .bar:nth-child(3) {
    animation-delay: 0.4s;
  }

  @keyframes eq-bounce {
    0%,
    100% {
      height: 4px;
    }
    50% {
      height: 16px;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .equalizer.playing .bar {
      animation: none;
      height: 10px;
    }
  }
</style>
