<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import NowPlaying from "./NowPlaying.svelte";

  const { Story } = defineMeta({
    title: "Components/NowPlaying",
    tags: ["autodocs"],
    parameters: {
      layout: "fullscreen",
    },
  });
</script>

<Story name="Playing">
  <div style="width: 390px;">
    <NowPlaying
      title="Bohemian Rhapsody"
      artist="Queen"
      singer="Alice"
      elapsed={154}
      duration={245}
    />
  </div>
</Story>

<Story name="Playing No Singer">
  <div style="width: 390px;">
    <NowPlaying
      title="Dancing Queen"
      artist="ABBA"
      elapsed={42}
      duration={231}
    />
  </div>
</Story>

<Story name="Long Title">
  <div style="width: 390px;">
    <NowPlaying
      title="Everybody Wants to Rule the World"
      artist="Tears for Fears"
      singer="Bob"
      elapsed={10}
      duration={254}
    />
  </div>
</Story>

<Story name="Near End">
  <div style="width: 390px;">
    <NowPlaying
      title="Take On Me"
      artist="a-ha"
      singer="Charlie"
      elapsed={220}
      duration={228}
    />
  </div>
</Story>

<Story name="Idle">
  <div style="width: 390px;">
    <NowPlaying />
  </div>
</Story>
