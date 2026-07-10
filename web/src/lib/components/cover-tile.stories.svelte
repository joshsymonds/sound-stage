<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import CoverTile from "./CoverTile.svelte";

  const { Story } = defineMeta({
    title: "Components/CoverTile",
    tags: ["autodocs"],
    parameters: {
      layout: "padded",
    },
  });
</script>

<Story name="With Cover">
  <div style="width: 128px;">
    <CoverTile
      song={{ id: "1", title: "Dancing Queen", artist: "ABBA" }}
      onqueue={() => {}}
    />
  </div>
</Story>

<Story name="Fallback Initials">
  <div style="width: 128px;">
    <CoverTile song={{ id: "", title: "Take On Me", artist: "a-ha" }} onqueue={() => {}} />
  </div>
</Story>

<Story name="Two-Word Artist Fallback">
  <div style="width: 128px;">
    <CoverTile
      song={{ id: "", title: "Everybody Wants to Rule the World", artist: "Tears for Fears" }}
      onqueue={() => {}}
    />
  </div>
</Story>

<Story name="Long Title and Artist">
  <div style="width: 128px;">
    <CoverTile
      song={{
        id: "",
        title: "Somebody That I Used to Know (Extended Radio Edit)",
        artist: "Gotye featuring Kimbra",
      }}
      onqueue={() => {}}
    />
  </div>
</Story>

<Story name="Grid">
  <div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; width: 400px;">
    <CoverTile song={{ id: "1", title: "Bohemian Rhapsody", artist: "Queen" }} onqueue={() => {}} />
    <CoverTile song={{ id: "", title: "Dancing Queen", artist: "ABBA" }} onqueue={() => {}} />
    <CoverTile song={{ id: "", title: "Take On Me", artist: "a-ha" }} onqueue={() => {}} />
  </div>
</Story>
