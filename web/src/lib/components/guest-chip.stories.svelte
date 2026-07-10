<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import GuestChip from "./GuestChip.svelte";

  const { Story } = defineMeta({
    title: "Components/GuestChip",
    tags: ["autodocs"],
    parameters: {
      layout: "padded",
    },
  });
</script>

<Story name="Default">
  <GuestChip name="Alice" />
</Story>

<Story name="With Name">
  <GuestChip name="Alice" showName />
</Story>

<Story name="Multiple Guests">
  <div style="display: flex; gap: 8px;">
    <GuestChip name="Alice" showName />
    <GuestChip name="Bob" showName />
    <GuestChip name="Charlie Brown" showName />
  </div>
</Story>
