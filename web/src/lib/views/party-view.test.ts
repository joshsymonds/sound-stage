import { cleanup, render, screen } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import PartyView from "./PartyView.svelte";

describe("PartyView", () => {
  afterEach(cleanup);

  it("renders the queue and party chips", () => {
    render(PartyView, {
      props: {
        queue: [
          {
            position: 1,
            song: { id: "1", title: "Dancing Queen", artist: "ABBA" },
            guest: "Alice",
            isNext: true,
          },
          {
            position: 2,
            song: { id: "2", title: "Take On Me", artist: "a-ha" },
            guest: "Bob",
            isNext: false,
          },
        ],
        guestName: "Bob",
        onremove: vi.fn(),
        onremoveperson: vi.fn(),
        onbrowse: vi.fn(),
      },
    });

    expect(screen.getByText("Dancing Queen")).toBeInTheDocument();
    expect(screen.getAllByText("Alice").length).toBeGreaterThan(0);
  });

  it("fires onremove when a guest removes their own queued song", async () => {
    const user = userEvent.setup();
    const handleRemove = vi.fn();
    render(PartyView, {
      props: {
        queue: [
          {
            position: 1,
            song: { id: "1", title: "Dancing Queen", artist: "ABBA" },
            guest: "Bob",
            isNext: true,
          },
        ],
        guestName: "Bob",
        onremove: handleRemove,
        onremoveperson: vi.fn(),
        onbrowse: vi.fn(),
      },
    });

    await user.click(screen.getByRole("button", { name: /remove your song/i }));
    expect(handleRemove).toHaveBeenCalledWith(1);
  });

  it("renders a guest chip avatar for each person in the party chips row", () => {
    const { container } = render(PartyView, {
      props: {
        queue: [
          {
            position: 1,
            song: { id: "1", title: "Dancing Queen", artist: "ABBA" },
            guest: "Alice",
            isNext: true,
          },
        ],
        guestName: "Bob",
        onremove: vi.fn(),
        onremoveperson: vi.fn(),
        onbrowse: vi.fn(),
      },
    });

    expect(container.querySelector(".people-chip .guest-chip")).not.toBeNull();
  });

  it("shows the up-next banner when the guest's own song is at the head of the queue", () => {
    render(PartyView, {
      props: {
        queue: [
          {
            position: 1,
            song: { id: "1", title: "Dancing Queen", artist: "ABBA" },
            guest: "Alice",
            isNext: true,
          },
        ],
        guestName: "Alice",
        onremove: vi.fn(),
        onremoveperson: vi.fn(),
        onbrowse: vi.fn(),
      },
    });

    expect(screen.getByText("You're up next, Alice! 🎤")).toBeInTheDocument();
  });

  it("does not show the up-next banner when someone else is at the head of the queue", () => {
    render(PartyView, {
      props: {
        queue: [
          {
            position: 1,
            song: { id: "1", title: "Dancing Queen", artist: "ABBA" },
            guest: "Bob",
            isNext: true,
          },
        ],
        guestName: "Alice",
        onremove: vi.fn(),
        onremoveperson: vi.fn(),
        onbrowse: vi.fn(),
      },
    });

    expect(screen.queryByText(/you're up next/i)).toBeNull();
  });

  it("does not show the up-next banner when the queue is empty", () => {
    render(PartyView, {
      props: {
        queue: [],
        guestName: "Alice",
        onremove: vi.fn(),
        onremoveperson: vi.fn(),
        onbrowse: vi.fn(),
      },
    });

    expect(screen.queryByText(/you're up next/i)).toBeNull();
  });

  it("passes a wait estimate through to each queue item", () => {
    render(PartyView, {
      props: {
        queue: [
          {
            position: 1,
            song: { id: "1", title: "Dancing Queen", artist: "ABBA" },
            guest: "Alice",
            isNext: true,
          },
          {
            position: 2,
            song: { id: "2", title: "Take On Me", artist: "a-ha" },
            guest: "Bob",
            isNext: false,
          },
        ],
        guestName: "Bob",
        onremove: vi.fn(),
        onremoveperson: vi.fn(),
        onbrowse: vi.fn(),
      },
    });

    expect(screen.getByText("up next")).toBeInTheDocument();
    // (2-1) * 3.5 * 60 = 210s -> 3.5min -> rounds to 4
    expect(screen.getByText("~4 min")).toBeInTheDocument();
  });

  it("shows a celebration overlay when the guest's song starts playing and auto-dismisses it", async () => {
    vi.useFakeTimers();
    try {
      const { rerender } = render(PartyView, {
        props: {
          queue: [
            {
              position: 1,
              song: { id: "1", title: "Dancing Queen", artist: "ABBA" },
              guest: "Alice",
              isNext: true,
            },
          ],
          guestName: "Alice",
          onremove: vi.fn(),
          onremoveperson: vi.fn(),
          onbrowse: vi.fn(),
        },
      });

      expect(screen.queryByText("You're on, Alice! 🎤")).toBeNull();

      await rerender({
        queue: [],
        nowPlaying: {
          id: "1",
          title: "Dancing Queen",
          artist: "ABBA",
          elapsed: 0,
          duration: 200,
        },
      });

      expect(screen.getByText("You're on, Alice! 🎤")).toBeInTheDocument();

      vi.advanceTimersByTime(4000);
      await vi.waitFor(() => {
        expect(screen.queryByText("You're on, Alice! 🎤")).toBeNull();
      });
    } finally {
      vi.useRealTimers();
    }
  });

  it("does not show a celebration overlay when a different guest's song starts playing", async () => {
    const { rerender } = render(PartyView, {
      props: {
        queue: [
          {
            position: 1,
            song: { id: "1", title: "Dancing Queen", artist: "ABBA" },
            guest: "Bob",
            isNext: true,
          },
        ],
        guestName: "Alice",
        onremove: vi.fn(),
        onremoveperson: vi.fn(),
        onbrowse: vi.fn(),
      },
    });

    await rerender({
      queue: [],
      nowPlaying: {
        id: "1",
        title: "Dancing Queen",
        artist: "ABBA",
        elapsed: 0,
        duration: 200,
      },
    });

    expect(screen.queryByText(/you're on/i)).toBeNull();
  });
});
