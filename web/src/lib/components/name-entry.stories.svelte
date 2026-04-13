<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import NameEntry from "./NameEntry.svelte";

  const { Story } = defineMeta({
    title: "Screens/NameEntry",
    tags: ["autodocs"],
    parameters: {
      layout: "fullscreen",
    },
  });
</script>

<Story name="Default">
  <div style="width: 390px; height: 844px;">
    <NameEntry onsubmit={(name) => console.log("Joined as:", name)} />
  </div>
</Story>
