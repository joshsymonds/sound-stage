<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import PartyView from "./PartyView.svelte";

  const { Story } = defineMeta({
    title: "Views/PartyView",
    tags: ["autodocs"],
    parameters: {
      layout: "fullscreen",
    },
  });
</script>

<Story name="With Queue">
  <div style="width: 390px;">
    <PartyView
      queue={[
        { position: 1, song: { id: "1", title: "Dancing Queen", artist: "ABBA" }, guest: "Alice", isNext: true },
        { position: 2, song: { id: "2", title: "Don't Stop Me Now", artist: "Queen" }, guest: "Bob", isNext: false },
        { position: 3, song: { id: "3", title: "Take On Me", artist: "a-ha" }, guest: "Alice", isNext: false },
      ]}
      guestName="Bob"
      onremove={() => {}}
      onremoveperson={() => {}}
      onbrowse={() => {}}
    />
  </div>
</Story>

<Story name="Empty Queue">
  <div style="width: 390px;">
    <PartyView
      queue={[]}
      guestName="Bob"
      onremove={() => {}}
      onremoveperson={() => {}}
      onbrowse={() => {}}
    />
  </div>
</Story>
