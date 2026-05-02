<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import SongCard from "./SongCard.svelte";

  const { Story } = defineMeta({
    title: "Components/SongCard",
    tags: ["autodocs"],
    parameters: {
      layout: "padded",
    },
  });
</script>

<Story name="Basic">
  <div style="width: 350px;">
    <SongCard title="Bohemian Rhapsody" artist="Queen" />
  </div>
</Story>

<Story name="With Edition">
  <div style="width: 350px;">
    <SongCard title="Waterloo" artist="ABBA" edition="ESC 1974" year={1974} />
  </div>
</Story>

<Story name="With Cover">
  <div style="width: 350px;">
    <SongCard
      title="Dancing Queen"
      artist="ABBA"
      coverUrl="https://placehold.co/96x96/ff2d7b/0a0a0f?text=DQ"
    />
  </div>
</Story>

<Story name="Interactive">
  <div style="width: 350px;">
    <SongCard
      title="Take On Me"
      artist="a-ha"
      year={1985}
      onclick={() => {}}
    />
  </div>
</Story>

<Story name="Long Title">
  <div style="width: 350px;">
    <SongCard
      title="Everybody Wants to Rule the World (Extended Mix)"
      artist="Tears for Fears"
      edition="Melodifestivalen 2026"
      year={2026}
    />
  </div>
</Story>

<Story name="Song List">
  <div style="width: 350px; display: flex; flex-direction: column; gap: 8px;">
    <SongCard title="Bohemian Rhapsody" artist="Queen" year={1975} onclick={() => {}} />
    <SongCard title="Dancing Queen" artist="ABBA" edition="ESC 1974" onclick={() => {}} />
    <SongCard title="Take On Me" artist="a-ha" year={1985} onclick={() => {}} />
    <SongCard title="Don't Stop Me Now" artist="Queen" year={1979} onclick={() => {}} />
    <SongCard title="Somebody That I Used to Know" artist="Gotye" year={2011} onclick={() => {}} />
  </div>
</Story>

<Story name="Unified Browse Results">
  <div style="width: 350px; display: flex; flex-direction: column; gap: 8px;">
    <SongCard
      title="Bohemian Rhapsody"
      artist="Queen"
      year={1975}
      onclick={() => {}}
      badge="instant"
    />
    <SongCard
      title="Dancing Queen"
      artist="ABBA"
      edition="ESC 1974"
      onclick={() => {}}
      badge="instant"
    />
    <SongCard
      title="Bohemian Rhapsody (Live Aid)"
      artist="Queen"
      onclick={() => {}}
    />
    <SongCard
      title="Don't Stop Me Now"
      artist="Queen"
      onclick={() => {}}
    />
  </div>
</Story>
