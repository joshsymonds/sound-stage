<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import FastScrollRail from "./FastScrollRail.svelte";

  const { Story } = defineMeta({
    title: "Components/FastScrollRail",
    tags: ["autodocs"],
    parameters: {
      layout: "fullscreen",
    },
  });
</script>

<Story name="Sparse Library">
  <div style="position: relative; height: 500px;">
    <FastScrollRail letters={new Set(["A", "B", "D", "Q", "T"])} onjump={() => {}} />
  </div>
</Story>

<Story name="Full Library">
  <div style="position: relative; height: 500px;">
    <FastScrollRail
      letters={new Set([
        "A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M",
        "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z",
      ])}
      onjump={() => {}}
    />
  </div>
</Story>
