import { cleanup, render, screen } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import BrowseView from "./BrowseView.svelte";

const songs = [{ id: "1", title: "Bohemian Rhapsody", artist: "Queen" }];

describe("BrowseView", () => {
  afterEach(cleanup);

  it("renders library songs", () => {
    render(BrowseView, {
      props: {
        songs,
        value: "",
        searching: false,
        loadingSongs: false,
        dedupedUSDB: [],
        downloadingIds: new Set<number>(),
        oninput: vi.fn(),
        onqueue: vi.fn(),
        ondownload: vi.fn(),
      },
    });

    expect(screen.getByText("Bohemian Rhapsody")).toBeInTheDocument();
  });

  it("fires onqueue when a library song is tapped", async () => {
    const user = userEvent.setup();
    const handleQueue = vi.fn();
    render(BrowseView, {
      props: {
        songs,
        value: "",
        searching: false,
        loadingSongs: false,
        dedupedUSDB: [],
        downloadingIds: new Set<number>(),
        oninput: vi.fn(),
        onqueue: handleQueue,
        ondownload: vi.fn(),
      },
    });

    await user.click(screen.getByText("Bohemian Rhapsody"));
    expect(handleQueue).toHaveBeenCalledWith(songs[0]);
  });
});
