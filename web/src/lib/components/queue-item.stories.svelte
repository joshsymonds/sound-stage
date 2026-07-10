<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import QueueItem from "./QueueItem.svelte";

  const { Story } = defineMeta({
    title: "Components/QueueItem",
    tags: ["autodocs"],
    parameters: {
      layout: "padded",
    },
  });
</script>

<Story name="Default">
  <div style="width: 350px;">
    <QueueItem
      position={2}
      title="Don't Stop Me Now"
      artist="Queen"
      guest="Bob"
      waitText="~4 min"
    />
  </div>
</Story>

<Story name="Next Up">
  <div style="width: 350px;">
    <QueueItem
      position={1}
      title="Dancing Queen"
      artist="ABBA"
      guest="Alice"
      isNext
      waitText="up next"
    />
  </div>
</Story>

<Story name="Queue List">
  <div style="width: 350px; display: flex; flex-direction: column; gap: 8px;">
    <QueueItem
      position={1}
      title="Dancing Queen"
      artist="ABBA"
      guest="Alice"
      isNext
      waitText="up next"
    />
    <QueueItem
      position={2}
      title="Don't Stop Me Now"
      artist="Queen"
      guest="Bob"
      waitText="~4 min"
    />
    <QueueItem
      position={3}
      title="Take On Me"
      artist="a-ha"
      guest="Alice"
      waitText="~7 min"
    />
    <QueueItem
      position={4}
      title="Somebody That I Used to Know"
      artist="Gotye"
      guest="Charlie"
      waitText="~11 min"
    />
    <QueueItem
      position={5}
      title="Bohemian Rhapsody"
      artist="Queen"
      guest="Bob"
      waitText="~14 min"
    />
  </div>
</Story>
