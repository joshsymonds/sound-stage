<script module>
  import { defineMeta } from "@storybook/addon-svelte-csf";

  import BrowseView from "./BrowseView.svelte";

  const { Story } = defineMeta({
    title: "Views/BrowseView",
    tags: ["autodocs"],
    parameters: {
      layout: "fullscreen",
    },
  });

  const noDownloads = new Set();
</script>

<Story name="Library">
  <div style="width: 390px;">
    <BrowseView
      songs={[
        { id: "1", title: "Bohemian Rhapsody", artist: "Queen", year: 1975 },
        { id: "2", title: "Dancing Queen", artist: "ABBA", edition: "ESC 1974" },
      ]}
      value=""
      searching={false}
      loadingSongs={false}
      dedupedUSDB={[]}
      downloadingIds={noDownloads}
      oninput={() => {}}
      onqueue={() => {}}
      ondownload={() => {}}
    />
  </div>
</Story>

<Story name="Search Results">
  <div style="width: 390px;">
    <BrowseView
      songs={[{ id: "1", title: "Bohemian Rhapsody", artist: "Queen", year: 1975 }]}
      value="queen"
      searching={false}
      loadingSongs={false}
      dedupedUSDB={[{ id: 42, title: "Don't Stop Me Now", artist: "Queen", language: "English" }]}
      downloadingIds={noDownloads}
      oninput={() => {}}
      onqueue={() => {}}
      ondownload={() => {}}
    />
  </div>
</Story>

<Story name="Loading Library">
  <div style="width: 390px;">
    <BrowseView
      songs={[]}
      value=""
      searching={false}
      loadingSongs={true}
      dedupedUSDB={[]}
      downloadingIds={noDownloads}
      oninput={() => {}}
      onqueue={() => {}}
      ondownload={() => {}}
    />
  </div>
</Story>
