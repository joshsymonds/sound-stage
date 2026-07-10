import { cleanup, render, screen } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { Song } from "../types";

import ShelfRow from "./ShelfRow.svelte";

const songs: Song[] = [
  { id: "1", title: "Bohemian Rhapsody", artist: "Queen" },
  { id: "2", title: "Dancing Queen", artist: "ABBA" },
];

describe("ShelfRow", () => {
  afterEach(cleanup);

  it("renders the label and songs", () => {
    render(ShelfRow, { props: { label: "Recently Added", songs, onqueue: vi.fn() } });
    expect(screen.getByText("Recently Added")).toBeInTheDocument();
    expect(screen.getByText("Bohemian Rhapsody")).toBeInTheDocument();
    expect(screen.getByText("Dancing Queen")).toBeInTheDocument();
  });

  it("fires onqueue when a tile is tapped", async () => {
    const user = userEvent.setup();
    const handleQueue = vi.fn();
    render(ShelfRow, { props: { label: "Recently Added", songs, onqueue: handleQueue } });

    await user.click(screen.getByText("Bohemian Rhapsody"));
    expect(handleQueue).toHaveBeenCalledWith(songs[0]);
  });

  it("renders nothing when songs is empty", () => {
    const { container } = render(ShelfRow, { props: { label: "Duets", songs: [], onqueue: vi.fn() } });
    expect(container.querySelector(".shelf")).toBeNull();
    expect(container.textContent).toBe("");
  });
});
