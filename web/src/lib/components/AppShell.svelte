<script lang="ts">
  import { untrack } from "svelte";
  import type { Snippet } from "svelte";

  let {
    activeTab = "playing",
    onnavigate,
    banner,
    headerEnd,
    children,
    queueBadge,
  }: {
    activeTab?: "playing" | "queue" | "browse";
    onnavigate?: (tab: string) => void;
    banner?: Snippet;
    headerEnd?: Snippet;
    children: Snippet;
    queueBadge?: number;
  } = $props();

  const tabs = [
    { id: "playing", label: "Now Playing" },
    { id: "queue", label: "Party" },
    { id: "browse", label: "Browse" },
  ] as const;

  // Remount key for the badge: incremented only when the count INCREASES,
  // so {#key} replays the bump keyframe on new songs but not on removals.
  // untrack around the previous-value read keeps the effect from
  // re-triggering itself (same idiom as PartyView's lastQueue tracking).
  let bumpKey = $state(0);
  let lastBadge = $state(0);
  $effect(() => {
    const next = queueBadge ?? 0;
    const previous = untrack(() => lastBadge);
    if (next > previous) {
      bumpKey = untrack(() => bumpKey) + 1;
    }
    lastBadge = next;
  });
</script>

<div class="app-shell">
  {#if banner}
    {@render banner()}
  {/if}

  <header class="header">
    <span class="logo">SoundStage</span>
    {#if headerEnd}
      <span class="header-end">
        {@render headerEnd()}
      </span>
    {/if}
  </header>

  <main class="content">
    {@render children()}
  </main>

  <nav class="nav">
    {#each tabs as tab (tab.id)}
      <button
        class="nav-item"
        class:active={activeTab === tab.id}
        type="button"
        onclick={() => onnavigate?.(tab.id)}
      >
        {tab.label}
        {#if tab.id === "queue" && queueBadge !== undefined && queueBadge > 0}
          {#key bumpKey}
            <span class="nav-badge">{queueBadge}</span>
          {/key}
        {/if}
      </button>
    {/each}
  </nav>
</div>

<style>
  .app-shell {
    display: flex;
    flex-direction: column;
    /* Use exact viewport height (not min-height) so the inner .content
       overflow:auto contains the scroll instead of pushing .nav off-screen.
       min-height would let the column grow past the viewport when content
       is tall, defeating the bottom-nav anchor. */
    height: 100vh;
    height: 100dvh;
    background: var(--color-bg);
    overflow: hidden;
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-md);
    border-bottom: 1px solid var(--color-border-subtle);
    flex-shrink: 0;
    position: relative;
  }

  .header-end {
    position: absolute;
    right: var(--space-md);
    top: 50%;
    transform: translateY(-50%);
    display: flex;
    align-items: center;
  }

  .logo {
    font-size: 1.125rem;
    font-weight: 800;
    color: var(--color-pink);
    text-shadow: var(--glow-text-pink);
    letter-spacing: -0.01em;
  }

  .content {
    flex: 1;
    /* min-height: 0 is the classic fix that lets a flex child actually
       scroll instead of growing to its intrinsic content height. */
    min-height: 0;
    overflow-y: auto;
    -webkit-overflow-scrolling: touch;
  }

  .nav {
    display: flex;
    border-top: 1px solid var(--color-border-subtle);
    background: var(--color-surface);
    flex-shrink: 0;
    padding-bottom: env(safe-area-inset-bottom, 0px);
  }

  .nav-item {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-md) var(--space-sm);
    font-family: var(--font-body);
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--color-text-muted);
    background: none;
    border: none;
    cursor: pointer;
    transition: color var(--transition-fast);
    position: relative;
  }

  .nav-item:hover {
    color: var(--color-text-dim);
  }

  .nav-item.active {
    color: var(--color-pink);
    font-weight: 600;
  }

  .nav-item.active::after {
    content: "";
    position: absolute;
    top: 0;
    left: 20%;
    right: 20%;
    height: 2px;
    background: var(--color-pink);
    box-shadow: 0 0 6px rgba(255, 45, 123, 0.5);
    border-radius: var(--radius-full);
  }

  .nav-badge {
    position: absolute;
    top: 6px;
    right: 18%;
    min-width: 18px;
    height: 18px;
    padding: 0 5px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: var(--color-pink);
    color: white;
    font-size: 0.6875rem;
    font-weight: 700;
    border-radius: var(--radius-full);
    box-shadow: var(--glow-pink);
    animation: badge-bump 300ms ease;
  }

  @keyframes badge-bump {
    0% {
      transform: scale(1);
    }
    40% {
      transform: scale(1.35);
    }
    100% {
      transform: scale(1);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .nav-badge {
      animation: none;
    }
  }
</style>
