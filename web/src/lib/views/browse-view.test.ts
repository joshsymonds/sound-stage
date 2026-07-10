import { cleanup, render, screen } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { Song } from "../types";

import BrowseView from "./BrowseView.svelte";

const songs: Song[] = [
  {
    id: "1",
    title: "Bohemian Rhapsody",
    artist: "Queen",
    year: 1975,
    genre: "Rock",
    addedAt: "2024-01-01",
  },
  {
    id: "2",
    title: "Dancing Queen",
    artist: "ABBA",
    year: 1976,
    genre: "Pop",
    addedAt: "2024-02-01",
  },
  { id: "3", title: "Under Pressure", artist: "Queen", duet: true, addedAt: "2024-03-01" },
];

const baseProps = {
  value: "",
  searching: false,
  loadingSongs: false,
  dedupedUSDB: [],
  downloadingIds: new Set<number>(),
  oninput: vi.fn(),
  onqueue: vi.fn(),
  ondownload: vi.fn(),
};

describe("BrowseView", () => {
  afterEach(cleanup);

  it("renders shelves with mock songs", () => {
    render(BrowseView, { props: { ...baseProps, songs } });
    expect(screen.getByText("Recently Added")).toBeInTheDocument();
    expect(screen.getByText("'70s")).toBeInTheDocument();
    expect(screen.getByText("Duets")).toBeInTheDocument();
    expect(screen.getByText("Pop")).toBeInTheDocument();
    expect(screen.getByText("Rock")).toBeInTheDocument();
  });

  it("does not render a genre shelf with no matching songs", () => {
    render(BrowseView, { props: { ...baseProps, songs } });
    expect(screen.queryByText("Soundtracks")).not.toBeInTheDocument();
  });

  it("renders the All Songs crate with every song", () => {
    const { container } = render(BrowseView, { props: { ...baseProps, songs } });
    expect(screen.getByText("All Songs")).toBeInTheDocument();
    expect(container.querySelectorAll(".crate-grid .cover-tile")).toHaveLength(songs.length);
  });

  it("anchors the first song per letter in the crate for fast-scroll", () => {
    const { container } = render(BrowseView, { props: { ...baseProps, songs } });
    expect(container.querySelector("#crate-A")).toBeInTheDocument();
    expect(container.querySelector("#crate-Q")).toBeInTheDocument();
  });

  it("renders the fast-scroll rail while not searching", () => {
    const { container } = render(BrowseView, { props: { ...baseProps, songs } });
    expect(container.querySelector(".rail")).toBeInTheDocument();
  });

  it("fires onqueue when a tile is tapped", async () => {
    const user = userEvent.setup();
    const handleQueue = vi.fn();
    render(BrowseView, { props: { ...baseProps, songs, onqueue: handleQueue } });

    const [tile] = screen.getAllByText("Dancing Queen");
    if (!tile) throw new Error("expected at least one Dancing Queen tile");
    await user.click(tile);
    expect(handleQueue).toHaveBeenCalledWith(songs[1]);
  });

  it("hides shelves and the crate while searching", () => {
    render(BrowseView, { props: { ...baseProps, songs, value: "queen" } });
    expect(screen.queryByText("Recently Added")).not.toBeInTheDocument();
    expect(screen.queryByText("All Songs")).not.toBeInTheDocument();
  });

  it("hides the fast-scroll rail while searching", () => {
    const { container } = render(BrowseView, { props: { ...baseProps, songs, value: "queen" } });
    expect(container.querySelector(".rail")).not.toBeInTheDocument();
  });

  it("shows library matches as tiles while searching", () => {
    const { container } = render(BrowseView, { props: { ...baseProps, songs, value: "queen" } });
    expect(container.querySelectorAll(".cover-tile").length).toBeGreaterThan(0);
  });

  it("still renders USDB results as SongCards while searching", async () => {
    const user = userEvent.setup();
    const handleDownload = vi.fn();
    const usdbResult = { id: 42, title: "Killer Queen", artist: "Queen", language: "English" };
    render(BrowseView, {
      props: {
        ...baseProps,
        songs,
        value: "queen",
        dedupedUSDB: [usdbResult],
        ondownload: handleDownload,
      },
    });

    await user.click(screen.getByText("Killer Queen"));
    expect(handleDownload).toHaveBeenCalledWith(usdbResult);
  });

  it("shows a loading state instead of shelves", () => {
    render(BrowseView, { props: { ...baseProps, songs: [], loadingSongs: true } });
    expect(screen.getByText("Loading…")).toBeInTheDocument();
    expect(screen.queryByText("All Songs")).not.toBeInTheDocument();
  });

  it("shows an empty-library prompt when there are no songs", () => {
    render(BrowseView, { props: { ...baseProps, songs: [] } });
    expect(screen.getByText(/Nothing downloaded yet/)).toBeInTheDocument();
  });
});
