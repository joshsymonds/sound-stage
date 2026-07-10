<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import EqualizerGlyph from "./EqualizerGlyph.svelte";

  const { Story } = defineMeta({
    title: "Components/EqualizerGlyph",
    tags: ["autodocs"],
    parameters: {
      layout: "padded",
    },
  });
</script>

<Story name="Playing">
  <div style="background: #0a0a0f; padding: 24px;">
    <EqualizerGlyph playing={true} />
  </div>
</Story>

<Story name="Paused">
  <div style="background: #0a0a0f; padding: 24px;">
    <EqualizerGlyph playing={false} />
  </div>
</Story>
