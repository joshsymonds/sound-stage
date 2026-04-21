import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";

import QueueItem from "./QueueItem.svelte";

describe("QueueItem", () => {
  afterEach(cleanup);

  it("renders position, title, artist, and guest", () => {
    render(QueueItem, {
      props: { position: 1, title: "Dancing Queen", artist: "ABBA", guest: "Alice" },
    });
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("Dancing Queen")).toBeInTheDocument();
    expect(screen.getByText("ABBA")).toBeInTheDocument();
    expect(screen.getByText("Alice")).toBeInTheDocument();
  });

  it("applies next styling when isNext is true", () => {
    const { container } = render(QueueItem, {
      props: { position: 1, title: "Test", artist: "Test", guest: "Bob", isNext: true },
    });
    const item = container.querySelector(".queue-item");
    expect(item?.classList.contains("next")).toBe(true);
  });

  it("does not apply next styling by default", () => {
    const { container } = render(QueueItem, {
      props: { position: 2, title: "Test", artist: "Test", guest: "Bob" },
    });
    const item = container.querySelector(".queue-item");
    expect(item?.classList.contains("next")).toBe(false);
  });

  it("renders different position numbers", () => {
    render(QueueItem, {
      props: { position: 42, title: "Test", artist: "Test", guest: "Charlie" },
    });
    expect(screen.getByText("42")).toBeInTheDocument();
  });

  it("renders a remove button when onremove is provided", () => {
    render(QueueItem, {
      props: {
        position: 1,
        title: "Test",
        artist: "Test",
        guest: "Alice",
        onremove: vi.fn(),
      },
    });
    expect(screen.getByLabelText("Remove your song")).toBeInTheDocument();
  });

  it("does not render a remove button when onremove is omitted", () => {
    render(QueueItem, {
      props: { position: 1, title: "Test", artist: "Test", guest: "Alice" },
    });
    expect(screen.queryByLabelText("Remove your song")).toBeNull();
  });

  it("calls onremove when the remove button is clicked", async () => {
    const onremove = vi.fn();
    render(QueueItem, {
      props: {
        position: 3,
        title: "Test",
        artist: "Test",
        guest: "Alice",
        onremove,
      },
    });
    await fireEvent.click(screen.getByLabelText("Remove your song"));
    expect(onremove).toHaveBeenCalledTimes(1);
  });
});
