import { dominantColor } from "$lib/color";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import NowPlaying from "./NowPlaying.svelte";

vi.mock("$lib/color", () => ({
  dominantColor: vi.fn().mockResolvedValue(null),
}));

describe("NowPlaying", () => {
  afterEach(cleanup);
  afterEach(() => {
    vi.mocked(dominantColor).mockClear();
  });

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
    const { container } = render(NowPlaying, {
      props: {
        title: "Test",
        artist: "Test",
        elapsed: 0,
        duration: 200,
        singer: "Alice",
      },
    });
    expect(container.querySelector(".singer")).toHaveTextContent("Alice");
  });

  it("does not render a singer line when singer is not provided", () => {
    const { container } = render(NowPlaying, {
      props: {
        title: "Test",
        artist: "Test",
        elapsed: 0,
        duration: 200,
      },
    });
    expect(container.querySelector(".singer")).not.toBeInTheDocument();
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

  it("animates the equalizer glyph while playing and not paused", () => {
    const { container } = render(NowPlaying, {
      props: {
        id: "abc123",
        title: "Test",
        artist: "Test",
        elapsed: 0,
        duration: 200,
        paused: false,
      },
    });
    expect(container.querySelector(".equalizer")).toHaveClass("playing");
  });

  it("stops the equalizer glyph animation while paused", () => {
    const { container } = render(NowPlaying, {
      props: {
        id: "abc123",
        title: "Test",
        artist: "Test",
        elapsed: 0,
        duration: 200,
        paused: true,
      },
    });
    expect(container.querySelector(".equalizer")).not.toHaveClass("playing");
  });

  it("renders a blurred cover backdrop derived from the song id", () => {
    const { container } = render(NowPlaying, {
      props: {
        id: "abc123",
        title: "Test",
        artist: "Test",
        elapsed: 0,
        duration: 200,
      },
    });
    const backdrop = container.querySelector<HTMLImageElement>(".backdrop");
    expect(backdrop).toBeInTheDocument();
    expect(backdrop?.src).toContain("/api/library/abc123/cover");
  });

  it("does not render a backdrop when no id is given", () => {
    const { container } = render(NowPlaying, {
      props: {
        title: "Test",
        artist: "Test",
        elapsed: 0,
        duration: 200,
      },
    });
    expect(container.querySelector(".backdrop")).not.toBeInTheDocument();
  });

  it("hides the backdrop after the cover image fails to load", async () => {
    const { container } = render(NowPlaying, {
      props: {
        id: "abc123",
        title: "Test",
        artist: "Test",
        elapsed: 0,
        duration: 200,
      },
    });
    const backdrop = container.querySelector<HTMLImageElement>(".backdrop")!;
    await fireEvent.error(backdrop);
    expect(container.querySelector(".backdrop")).not.toBeInTheDocument();
  });

  it("sets --hero-glow on the hero root once dominantColor resolves", async () => {
    vi.mocked(dominantColor).mockResolvedValueOnce("hsl(200 50% 45%)");
    const { container } = render(NowPlaying, {
      props: {
        id: "abc123",
        title: "Test",
        artist: "Test",
        elapsed: 0,
        duration: 200,
      },
    });

    await vi.waitFor(() => {
      expect(
        container.querySelector<HTMLElement>(".now-playing")!.style.getPropertyValue(
          "--hero-glow",
        ),
      ).toBe("hsl(200 50% 45%)");
    });
    expect(dominantColor).toHaveBeenCalledWith("/api/library/abc123/thumb");
  });

  it("does not set --hero-glow when dominantColor resolves null", async () => {
    vi.mocked(dominantColor).mockResolvedValueOnce(null);
    const { container } = render(NowPlaying, {
      props: {
        id: "abc123",
        title: "Test",
        artist: "Test",
        elapsed: 0,
        duration: 200,
      },
    });

    await vi.waitFor(() => {
      expect(dominantColor).toHaveBeenCalled();
    });
    const hero = container.querySelector<HTMLElement>(".now-playing")!;
    expect(hero.style.getPropertyValue("--hero-glow")).toBe("");
  });
});
