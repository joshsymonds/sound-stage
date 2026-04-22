<script lang="ts">
  import type { Snippet } from "svelte";

  let {
    activeTab = "playing",
    onnavigate,
    banner,
    headerEnd,
    children,
  }: {
    activeTab?: "playing" | "queue" | "browse";
    onnavigate?: (tab: string) => void;
    banner?: Snippet;
    headerEnd?: Snippet;
    children: Snippet;
  } = $props();

  const tabs = [
    { id: "playing", label: "Now Playing" },
    { id: "queue", label: "Party" },
    { id: "browse", label: "Browse" },
  ] as const;
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
</style>
