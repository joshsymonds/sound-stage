<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import NowPlayingView from "./NowPlayingView.svelte";

  const { Story } = defineMeta({
    title: "Views/NowPlayingView",
    tags: ["autodocs"],
    parameters: {
      layout: "fullscreen",
    },
  });
</script>

<Story name="Playing With Up Next">
  <div style="width: 390px;">
    <NowPlayingView
      nowPlaying={{ id: "1", title: "Bohemian Rhapsody", artist: "Queen", elapsed: 154, duration: 245, singer: "Alice" }}
      displayedElapsed={154}
      paused={false}
      queue={[
        { position: 1, song: { id: "2", title: "Dancing Queen", artist: "ABBA" }, guest: "Alice", isNext: true },
        { position: 2, song: { id: "3", title: "Take On Me", artist: "a-ha" }, guest: "Bob", isNext: false },
      ]}
      guestName="Bob"
      onpause={() => {}}
      onresume={() => {}}
      onremove={() => {}}
      onbrowse={() => {}}
    />
  </div>
</Story>

<Story name="Empty Queue">
  <div style="width: 390px;">
    <NowPlayingView
      nowPlaying={null}
      displayedElapsed={0}
      paused={false}
      queue={[]}
      guestName="Bob"
      onpause={() => {}}
      onresume={() => {}}
      onremove={() => {}}
      onbrowse={() => {}}
    />
  </div>
</Story>
