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

  const crateSongs = [
    {
      id: "1",
      title: "Bohemian Rhapsody",
      artist: "Queen",
      year: 1975,
      genre: "Rock",
      addedAt: "2026-06-01",
    },
    {
      id: "2",
      title: "Dancing Queen",
      artist: "ABBA",
      edition: "ESC 1974",
      year: 1976,
      genre: "Pop",
      addedAt: "2026-05-15",
    },
    { id: "3", title: "Take On Me", artist: "a-ha", year: 1985, genre: "Synth-Pop" },
    { id: "4", title: "Don't Stop Me Now", artist: "Queen", year: 1979, genre: "Rock" },
    {
      id: "5",
      title: "Somebody That I Used to Know",
      artist: "Gotye",
      year: 2011,
      genre: "Indie Pop",
      addedAt: "2026-06-10",
    },
    { id: "6", title: "Under Pressure", artist: "Queen", duet: true, year: 1981 },
    { id: "7", title: "A Whole New World", artist: "Disney", genre: "Disney", year: 1992 },
    { id: "8", title: "Smells Like Teen Spirit", artist: "Nirvana", genre: "Grunge", year: 1991 },
  ];
</script>

<Story name="Record Crate">
  <div style="width: 390px;">
    <BrowseView
      songs={crateSongs}
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
      songs={crateSongs}
      value="queen"
      searching={false}
      loadingSongs={false}
      dedupedUSDB={[{ id: 42, title: "Killer Queen", artist: "Queen", language: "English" }]}
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

<Story name="Empty Library">
  <div style="width: 390px;">
    <BrowseView
      songs={[]}
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
