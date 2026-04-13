<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  const { Story } = defineMeta({
    title: "Design System",
    tags: ["autodocs"],
    parameters: {
      layout: "padded",
    },
  });

  const colors = [
    { name: "--color-bg", label: "Background" },
    { name: "--color-surface", label: "Surface" },
    { name: "--color-surface-raised", label: "Surface Raised" },
    { name: "--color-pink", label: "Pink (Primary)" },
    { name: "--color-pink-light", label: "Pink Light" },
    { name: "--color-pink-dim", label: "Pink Dim" },
    { name: "--color-cyan", label: "Cyan" },
    { name: "--color-purple", label: "Purple" },
    { name: "--color-green", label: "Green" },
    { name: "--color-yellow", label: "Yellow" },
    { name: "--color-red", label: "Red" },
    { name: "--color-text", label: "Text" },
    { name: "--color-text-dim", label: "Text Dim" },
    { name: "--color-text-muted", label: "Text Muted" },
  ];

  const typeScale = [
    { label: "Display", size: "2.5rem", weight: "800", tracking: "-0.02em" },
    { label: "Heading 1", size: "1.75rem", weight: "700", tracking: "-0.01em" },
    { label: "Heading 2", size: "1.25rem", weight: "600", tracking: "0" },
    { label: "Body", size: "1rem", weight: "400", tracking: "0" },
    { label: "Body Small", size: "0.875rem", weight: "400", tracking: "0" },
    { label: "Caption", size: "0.75rem", weight: "500", tracking: "0.02em" },
    { label: "Overline", size: "0.6875rem", weight: "600", tracking: "0.08em" },
  ];
</script>

<!-- Color Palette -->
<Story name="Color Palette">
  <div class="story-container">
    <div class="section-label">COLOR PALETTE</div>
    <div class="color-grid">
      {#each colors as c (c.name)}
        <div class="color-card">
          <div
            class="color-swatch"
            style="background: var({c.name});"
          ></div>
          <div class="color-label">{c.label}</div>
          <div class="color-var">{c.name}</div>
        </div>
      {/each}
    </div>

    <div class="section-label" style="margin-top: 32px;">BORDER COLORS</div>
    <div class="color-grid">
      <div class="color-card">
        <div class="color-swatch" style="background: var(--color-border); border: 1px solid rgba(255,255,255,0.1);"></div>
        <div class="color-label">Border</div>
        <div class="color-var">--color-border</div>
      </div>
      <div class="color-card">
        <div class="color-swatch" style="background: var(--color-border-subtle); border: 1px solid rgba(255,255,255,0.1);"></div>
        <div class="color-label">Border Subtle</div>
        <div class="color-var">--color-border-subtle</div>
      </div>
    </div>
  </div>
</Story>

<!-- Typography Scale -->
<Story name="Typography">
  <div class="story-container">
    <div class="section-label">TYPOGRAPHY — INTER</div>
    <div class="type-list">
      {#each typeScale as t (t.label)}
        <div class="type-row">
          <div class="type-meta">
            <span class="type-name">{t.label}</span>
            <span class="type-spec">{t.size} / {t.weight} / {t.tracking}</span>
          </div>
          <div
            class="type-sample"
            style="font-size: {t.size}; font-weight: {t.weight}; letter-spacing: {t.tracking};"
          >
            SoundStage Karaoke
          </div>
        </div>
      {/each}
    </div>
  </div>
</Story>

<!-- Neon Glow Effects -->
<Story name="Neon Glow">
  <div class="story-container">
    <div class="section-label">NEON GLOW EFFECTS</div>
    <div class="glow-grid">
      <div class="glow-card">
        <div class="glow-box" style="box-shadow: var(--glow-pink);"></div>
        <div class="glow-label">--glow-pink</div>
        <div class="glow-desc">Buttons, cards on hover</div>
      </div>
      <div class="glow-card">
        <div class="glow-box glow-breathe" style="box-shadow: var(--glow-pink-strong);"></div>
        <div class="glow-label">--glow-pink-strong</div>
        <div class="glow-desc">Now playing, active states</div>
      </div>
      <div class="glow-card">
        <div class="glow-box" style="box-shadow: var(--glow-cyan);"></div>
        <div class="glow-label">--glow-cyan</div>
        <div class="glow-desc">Info, secondary accents</div>
      </div>
      <div class="glow-card">
        <div class="glow-box" style="box-shadow: var(--glow-purple);"></div>
        <div class="glow-label">--glow-purple</div>
        <div class="glow-desc">Queue position, badges</div>
      </div>
    </div>

    <div class="section-label" style="margin-top: 32px;">TEXT GLOW</div>
    <div class="text-glow-row">
      <span class="text-glow-sample" style="color: var(--color-pink); text-shadow: var(--glow-text-pink);">
        Now Playing
      </span>
      <span class="text-glow-var">--glow-text-pink</span>
    </div>
  </div>
</Story>

<!-- Buttons & Interactive States -->
<Story name="Buttons">
  <div class="story-container">
    <div class="section-label">BUTTONS</div>
    <div class="button-row">
      <button class="btn btn-primary">Queue Song</button>
      <button class="btn btn-secondary">Browse Library</button>
      <button class="btn btn-ghost">Cancel</button>
    </div>

    <div class="section-label" style="margin-top: 32px;">BUTTON SIZES</div>
    <div class="button-row">
      <button class="btn btn-primary btn-sm">Small</button>
      <button class="btn btn-primary">Default</button>
      <button class="btn btn-primary btn-lg">Large</button>
    </div>

    <div class="section-label" style="margin-top: 32px;">STATES</div>
    <div class="button-row">
      <button class="btn btn-primary">Default</button>
      <button class="btn btn-primary" style="box-shadow: var(--glow-pink-strong);">Hover / Focus</button>
      <button class="btn btn-primary" disabled>Disabled</button>
    </div>
  </div>
</Story>

<!-- App Mockup -->
<Story name="App Mockup">
  <div class="mockup-container">
    <!-- Header -->
    <header class="mockup-header">
      <span class="mockup-logo">SoundStage</span>
    </header>

    <!-- Now Playing -->
    <div class="mockup-now-playing">
      <div class="mockup-overline">NOW PLAYING</div>
      <div class="mockup-song-title">Bohemian Rhapsody</div>
      <div class="mockup-song-artist">Queen</div>
      <div class="mockup-progress">
        <div class="mockup-progress-bar">
          <div class="mockup-progress-fill" style="width: 62%;"></div>
        </div>
        <div class="mockup-progress-times">
          <span>2:34</span>
          <span>4:05</span>
        </div>
      </div>
    </div>

    <!-- Queue -->
    <div class="mockup-queue">
      <div class="mockup-overline">UP NEXT</div>
      {#each [
        { pos: 1, title: "Dancing Queen", artist: "ABBA", guest: "Guest A" },
        { pos: 2, title: "Don't Stop Me Now", artist: "Queen", guest: "Guest B" },
        { pos: 3, title: "Take On Me", artist: "a-ha", guest: "Guest A" },
      ] as song (song.pos)}
        <div class="mockup-queue-item" class:next={song.pos === 1}>
          <span class="mockup-queue-pos" class:next={song.pos === 1}>{song.pos}</span>
          <div class="mockup-queue-info">
            <span class="mockup-queue-title">{song.title}</span>
            <span class="mockup-queue-artist">{song.artist}</span>
          </div>
          <span class="mockup-queue-guest">{song.guest}</span>
        </div>
      {/each}
    </div>

    <!-- Action bar -->
    <div class="mockup-action-bar">
      <button class="btn btn-primary" style="width: 100%;">Browse & Queue Songs</button>
    </div>
  </div>
</Story>

<style>
  /* ── Story containers ──────────────────────────────────── */
  .story-container {
    width: 640px;
    padding: 24px;
  }

  .section-label {
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    color: var(--color-pink);
    text-shadow: var(--glow-text-pink);
    margin-bottom: 16px;
  }

  /* ── Color palette ─────────────────────────────────────── */
  .color-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 12px;
  }

  .color-card {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .color-swatch {
    width: 100%;
    height: 48px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--color-border-subtle);
  }

  .color-label {
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--color-text);
  }

  .color-var {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    font-family: monospace;
  }

  /* ── Typography ────────────────────────────────────────── */
  .type-list {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .type-row {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding-bottom: 16px;
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .type-meta {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
  }

  .type-name {
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.04em;
    color: var(--color-pink);
  }

  .type-spec {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    font-family: monospace;
  }

  .type-sample {
    color: var(--color-text);
  }

  /* ── Neon glow ─────────────────────────────────────────── */
  .glow-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 16px;
  }

  .glow-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
  }

  .glow-box {
    width: 80px;
    height: 48px;
    border-radius: var(--radius-md);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
  }

  .glow-breathe {
    animation: glow-breathe 2s ease-in-out infinite;
  }

  .glow-label {
    font-size: 0.6875rem;
    color: var(--color-text-dim);
    font-family: monospace;
  }

  .glow-desc {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    text-align: center;
  }

  .text-glow-row {
    display: flex;
    align-items: baseline;
    gap: 16px;
  }

  .text-glow-sample {
    font-size: 1.75rem;
    font-weight: 700;
  }

  .text-glow-var {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    font-family: monospace;
  }

  /* ── Buttons ───────────────────────────────────────────── */
  .button-row {
    display: flex;
    gap: 12px;
    align-items: center;
  }

  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-sm);
    padding: 10px 20px;
    border-radius: var(--radius-md);
    font-family: var(--font-body);
    font-size: 0.875rem;
    font-weight: 600;
    cursor: pointer;
    transition: all var(--transition-normal);
    border: none;
    outline: none;
  }

  .btn:focus-visible {
    outline: 2px solid var(--color-pink);
    outline-offset: 2px;
  }

  .btn-primary {
    background: var(--color-pink);
    color: white;
    box-shadow: var(--glow-pink);
  }

  .btn-primary:hover {
    background: var(--color-pink-light);
    box-shadow: var(--glow-pink-strong);
  }

  .btn-primary:disabled {
    background: var(--color-pink-dim);
    opacity: 0.5;
    cursor: not-allowed;
    box-shadow: none;
  }

  .btn-secondary {
    background: var(--color-surface-raised);
    color: var(--color-text);
    border: 1px solid var(--color-border);
  }

  .btn-secondary:hover {
    background: var(--color-surface);
    border-color: var(--color-pink);
    box-shadow: var(--glow-pink);
  }

  .btn-ghost {
    background: transparent;
    color: var(--color-text-dim);
  }

  .btn-ghost:hover {
    color: var(--color-text);
    background: rgba(255, 255, 255, 0.04);
  }

  .btn-sm {
    padding: 6px 14px;
    font-size: 0.8125rem;
  }

  .btn-lg {
    padding: 14px 28px;
    font-size: 1rem;
  }

  /* ── App Mockup ────────────────────────────────────────── */
  .mockup-container {
    width: 100%;
    max-width: 390px;
    min-height: 667px;
    background: var(--color-bg);
    border-radius: var(--radius-lg);
    border: 1px solid var(--color-border-subtle);
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .mockup-header {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-md);
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .mockup-logo {
    font-size: 1.125rem;
    font-weight: 800;
    color: var(--color-pink);
    text-shadow: var(--glow-text-pink);
    letter-spacing: -0.01em;
  }

  .mockup-now-playing {
    padding: var(--space-lg);
    background: var(--color-surface);
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .mockup-overline {
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    color: var(--color-pink);
    text-shadow: var(--glow-text-pink);
    margin-bottom: var(--space-sm);
  }

  .mockup-song-title {
    font-size: 1.5rem;
    font-weight: 700;
    color: var(--color-text);
    margin-bottom: 2px;
  }

  .mockup-song-artist {
    font-size: 1rem;
    color: var(--color-text-dim);
    margin-bottom: var(--space-md);
  }

  .mockup-progress {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .mockup-progress-bar {
    width: 100%;
    height: 3px;
    background: var(--color-surface-raised);
    border-radius: var(--radius-full);
    overflow: hidden;
  }

  .mockup-progress-fill {
    height: 100%;
    background: var(--color-pink);
    border-radius: var(--radius-full);
    box-shadow: 0 0 6px rgba(255, 45, 123, 0.5);
  }

  .mockup-progress-times {
    display: flex;
    justify-content: space-between;
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }

  .mockup-queue {
    flex: 1;
    padding: var(--space-md) var(--space-lg);
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
  }

  .mockup-queue-item {
    display: flex;
    align-items: center;
    gap: var(--space-md);
    padding: var(--space-sm) var(--space-md);
    background: var(--color-surface);
    border-radius: var(--radius-md);
    border: 1px solid var(--color-border-subtle);
    transition: all var(--transition-normal);
  }

  .mockup-queue-item.next {
    border-color: var(--color-pink);
    box-shadow: var(--glow-pink);
  }

  .mockup-queue-pos {
    font-size: 0.875rem;
    font-weight: 700;
    color: var(--color-text-muted);
    width: 20px;
    text-align: center;
  }

  .mockup-queue-pos.next {
    color: var(--color-pink);
  }

  .mockup-queue-info {
    flex: 1;
    display: flex;
    flex-direction: column;
  }

  .mockup-queue-title {
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--color-text);
  }

  .mockup-queue-artist {
    font-size: 0.75rem;
    color: var(--color-text-dim);
  }

  .mockup-queue-guest {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
  }

  .mockup-action-bar {
    padding: var(--space-md) var(--space-lg) var(--space-lg);
  }
</style>
