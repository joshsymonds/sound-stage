<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import Button from "./Button.svelte";

  const { Story } = defineMeta({
    title: "Components/Button",
    tags: ["autodocs"],
    parameters: {
      layout: "padded",
    },
  });
</script>

<Story name="Primary">
  <Button>Queue Song</Button>
</Story>

<Story name="Secondary">
  <Button variant="secondary">Browse Library</Button>
</Story>

<Story name="Ghost">
  <Button variant="ghost">Cancel</Button>
</Story>

<Story name="Small">
  <Button size="sm">Small</Button>
</Story>

<Story name="Large">
  <Button size="lg">Large</Button>
</Story>

<Story name="Disabled">
  <Button disabled>Disabled</Button>
</Story>

<Story name="All Variants">
  <div style="display: flex; gap: 12px; align-items: center; flex-wrap: wrap;">
    <Button>Primary</Button>
    <Button variant="secondary">Secondary</Button>
    <Button variant="ghost">Ghost</Button>
  </div>
</Story>

<Story name="All Sizes">
  <div style="display: flex; gap: 12px; align-items: center;">
    <Button size="sm">Small</Button>
    <Button>Default</Button>
    <Button size="lg">Large</Button>
  </div>
</Story>
