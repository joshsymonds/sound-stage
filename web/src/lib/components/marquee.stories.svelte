<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import Marquee from "./Marquee.svelte";

  const { Story } = defineMeta({
    title: "Components/Marquee",
    tags: ["autodocs"],
    parameters: {
      layout: "padded",
    },
  });
</script>

<Story name="Fits Without Scrolling">
  <div style="width: 300px; background: #0a0a0f; color: #f0f0f5; padding: 16px; font-size: 1.5rem;">
    <Marquee>Dancing Queen</Marquee>
  </div>
</Story>

<Story name="Overflows And Scrolls">
  <div style="width: 200px; background: #0a0a0f; color: #f0f0f5; padding: 16px; font-size: 1.5rem;">
    <Marquee>Everybody Wants to Rule the World</Marquee>
  </div>
</Story>
