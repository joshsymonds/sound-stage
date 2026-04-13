import { cleanup, render, screen } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import NowPlaying from "./NowPlaying.svelte";

describe("NowPlaying", () => {
  afterEach(cleanup);

  it("renders song title and artist when playing", () => {
    render(NowPlaying, {
      props: {
        title: "Bohemian Rhapsody",
        artist: "Queen",
        elapsed: 154,
        duration: 245,
      },
    });
    expect(screen.getByText("Bohemian Rhapsody")).toBeInTheDocument();
    expect(screen.getByText("Queen")).toBeInTheDocument();
  });

  it("shows NOW PLAYING label when song is active", () => {
    render(NowPlaying, {
      props: {
        title: "Test",
        artist: "Test",
        elapsed: 0,
        duration: 200,
      },
    });
    expect(screen.getByText("NOW PLAYING")).toBeInTheDocument();
  });

  it("shows idle state when no song", () => {
    render(NowPlaying, { props: {} });
    expect(screen.getByText("No song playing")).toBeInTheDocument();
  });

  it("formats elapsed and duration as mm:ss", () => {
    render(NowPlaying, {
      props: {
        title: "Test",
        artist: "Test",
        elapsed: 94,
        duration: 245,
      },
    });
    expect(screen.getByText("1:34")).toBeInTheDocument();
    expect(screen.getByText("4:05")).toBeInTheDocument();
  });

  it("renders progress bar with correct fill percentage", () => {
    const { container } = render(NowPlaying, {
      props: {
        title: "Test",
        artist: "Test",
        elapsed: 100,
        duration: 200,
      },
    });
    const fill = container.querySelector(".progress-fill");
    expect(fill).toBeInTheDocument();
    expect((fill as HTMLElement).style.width).toBe("50%");
  });

  it("shows singer name when provided", () => {
    render(NowPlaying, {
      props: {
        title: "Test",
        artist: "Test",
        elapsed: 0,
        duration: 200,
        singer: "Alice",
      },
    });
    expect(screen.getByText("Alice")).toBeInTheDocument();
  });

  it("shows Pause button when playing and handlers provided", () => {
    render(NowPlaying, {
      props: {
        title: "Test",
        artist: "Test",
        elapsed: 0,
        duration: 200,
        onpause: vi.fn(),
        onresume: vi.fn(),
      },
    });
    expect(screen.getByRole("button", { name: "Pause" })).toBeInTheDocument();
  });

  it("shows PAUSED label and Resume button when paused", () => {
    render(NowPlaying, {
      props: {
        title: "Test",
        artist: "Test",
        elapsed: 50,
        duration: 200,
        paused: true,
        onpause: vi.fn(),
        onresume: vi.fn(),
      },
    });
    expect(screen.getByText("PAUSED")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Resume" })).toBeInTheDocument();
  });

  it("calls onpause when Pause clicked", async () => {
    const user = userEvent.setup();
    const handlePause = vi.fn();
    render(NowPlaying, {
      props: {
        title: "Test",
        artist: "Test",
        elapsed: 0,
        duration: 200,
        onpause: handlePause,
        onresume: vi.fn(),
      },
    });
    await user.click(screen.getByRole("button", { name: "Pause" }));
    expect(handlePause).toHaveBeenCalledOnce();
  });
});
