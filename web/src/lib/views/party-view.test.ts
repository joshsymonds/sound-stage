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
});
