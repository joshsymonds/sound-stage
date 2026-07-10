import { cleanup, render, screen } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import NowPlayingView from "./NowPlayingView.svelte";

describe("NowPlayingView", () => {
  afterEach(cleanup);

  it("renders the now-playing song and up-next queue", () => {
    render(NowPlayingView, {
      props: {
        nowPlaying: { id: "1", title: "Bohemian Rhapsody", artist: "Queen", elapsed: 60, duration: 240 },
        displayedElapsed: 60,
        paused: false,
        queue: [
          {
            position: 1,
            song: { id: "2", title: "Dancing Queen", artist: "ABBA" },
            guest: "Alice",
            isNext: true,
          },
        ],
        guestName: "Bob",
        onpause: vi.fn(),
        onresume: vi.fn(),
        onremove: vi.fn(),
        onbrowse: vi.fn(),
      },
    });

    expect(screen.getByText("Bohemian Rhapsody")).toBeInTheDocument();
    expect(screen.getByText("Dancing Queen")).toBeInTheDocument();
  });

  it("fires onbrowse when Browse Songs is clicked in the empty state", async () => {
    const user = userEvent.setup();
    const handleBrowse = vi.fn();
    render(NowPlayingView, {
      props: {
        nowPlaying: null,
        displayedElapsed: 0,
        paused: false,
        queue: [],
        guestName: "Bob",
        onpause: vi.fn(),
        onresume: vi.fn(),
        onremove: vi.fn(),
        onbrowse: handleBrowse,
      },
    });

    await user.click(screen.getByRole("button", { name: /browse songs/i }));
    expect(handleBrowse).toHaveBeenCalledOnce();
  });
});
