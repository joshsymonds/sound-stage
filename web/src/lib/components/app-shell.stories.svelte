<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import AppShell from "./AppShell.svelte";
  import NowPlaying from "./NowPlaying.svelte";
  import QueueItem from "./QueueItem.svelte";
  import SongCard from "./SongCard.svelte";
  import Button from "./Button.svelte";

  const { Story } = defineMeta({
    title: "Layouts/AppShell",
    tags: ["autodocs"],
    parameters: {
      layout: "fullscreen",
    },
  });
</script>

<Story name="Now Playing View">
  <div style="width: 390px; height: 844px; overflow: hidden;">
    <AppShell activeTab="playing">
      <NowPlaying
        title="Bohemian Rhapsody"
        artist="Queen"
        singer="Alice"
        elapsed={154}
        duration={245}
      />
      <div style="padding: var(--space-md) var(--space-lg);">
        <div style="font-size: 0.6875rem; font-weight: 600; letter-spacing: 0.08em; color: var(--color-pink); text-shadow: var(--glow-text-pink); margin-bottom: var(--space-sm);">UP NEXT</div>
        <div style="display: flex; flex-direction: column; gap: var(--space-sm);">
          <QueueItem position={1} title="Dancing Queen" artist="ABBA" guest="Bob" isNext />
          <QueueItem position={2} title="Take On Me" artist="a-ha" guest="Alice" />
          <QueueItem position={3} title="Don't Stop Me Now" artist="Queen" guest="Charlie" />
        </div>
      </div>
    </AppShell>
  </div>
</Story>

<Story name="Queue View">
  <div style="width: 390px; height: 844px; overflow: hidden;">
    <AppShell activeTab="queue">
      <div style="padding: var(--space-md) var(--space-lg);">
        <div style="font-size: 0.6875rem; font-weight: 600; letter-spacing: 0.08em; color: var(--color-pink); text-shadow: var(--glow-text-pink); margin-bottom: var(--space-sm);">QUEUE</div>
        <div style="display: flex; flex-direction: column; gap: var(--space-sm);">
          <QueueItem position={1} title="Dancing Queen" artist="ABBA" guest="Alice" isNext />
          <QueueItem position={2} title="Take On Me" artist="a-ha" guest="Bob" />
          <QueueItem position={3} title="Don't Stop Me Now" artist="Queen" guest="Alice" />
          <QueueItem position={4} title="Somebody That I Used to Know" artist="Gotye" guest="Charlie" />
          <QueueItem position={5} title="Bohemian Rhapsody" artist="Queen" guest="Bob" />
        </div>
      </div>
    </AppShell>
  </div>
</Story>

<Story name="Browse View">
  <div style="width: 390px; height: 844px; overflow: hidden;">
    <AppShell activeTab="browse">
      <div style="padding: var(--space-md) var(--space-lg);">
        <div style="padding: var(--space-sm) 0 var(--space-md);">
          <input
            type="text"
            placeholder="Search songs..."
            style="width: 100%; padding: 10px 16px; background: var(--color-surface); border: 1px solid var(--color-border-subtle); border-radius: var(--radius-md); color: var(--color-text); font-family: var(--font-body); font-size: 0.875rem; outline: none;"
          />
        </div>
        <div style="display: flex; flex-direction: column; gap: var(--space-sm);">
          <SongCard title="Bohemian Rhapsody" artist="Queen" year={1975} onclick={() => {}} />
          <SongCard title="Dancing Queen" artist="ABBA" edition="ESC 1974" onclick={() => {}} />
          <SongCard title="Take On Me" artist="a-ha" year={1985} onclick={() => {}} />
          <SongCard title="Don't Stop Me Now" artist="Queen" year={1979} onclick={() => {}} />
          <SongCard title="Waterloo" artist="ABBA" edition="ESC 1974" year={1974} onclick={() => {}} />
        </div>
      </div>
    </AppShell>
  </div>
</Story>

<Story name="Idle">
  <div style="width: 390px; height: 844px; overflow: hidden;">
    <AppShell activeTab="playing">
      <NowPlaying />
      <div style="padding: var(--space-lg); text-align: center;">
        <p style="color: var(--color-text-muted); font-size: 0.875rem; margin-bottom: var(--space-md);">Queue a song to get started!</p>
        <Button>Browse Songs</Button>
      </div>
    </AppShell>
  </div>
</Story>
