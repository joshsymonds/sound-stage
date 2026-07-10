<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import ShelfRow from "./ShelfRow.svelte";

  const { Story } = defineMeta({
    title: "Components/ShelfRow",
    tags: ["autodocs"],
    parameters: {
      layout: "fullscreen",
    },
  });
</script>

<Story name="Recently Added">
  <div style="width: 390px; padding: 16px;">
    <ShelfRow
      label="Recently Added"
      songs={[
        { id: "1", title: "Bohemian Rhapsody", artist: "Queen" },
        { id: "2", title: "Dancing Queen", artist: "ABBA" },
        { id: "3", title: "Take On Me", artist: "a-ha" },
        { id: "4", title: "Don't Stop Me Now", artist: "Queen" },
        { id: "5", title: "Somebody That I Used to Know", artist: "Gotye" },
      ]}
      onqueue={() => {}}
    />
  </div>
</Story>

<Story name="Empty">
  <div style="width: 390px; padding: 16px;">
    <ShelfRow label="Duets" songs={[]} onqueue={() => {}} />
  </div>
</Story>
