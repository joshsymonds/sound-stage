import { textSnippet } from "$lib/test-helpers";
import { cleanup, render, screen } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import AppShell from "./AppShell.svelte";

describe("AppShell", () => {
  afterEach(cleanup);

  it("renders SoundStage header", () => {
    render(AppShell, { props: { children: textSnippet("content") } });
    expect(screen.getByText("SoundStage")).toBeInTheDocument();
  });

  it("renders navigation items", () => {
    render(AppShell, { props: { children: textSnippet("content") } });
    expect(screen.getByRole("button", { name: /now playing/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /party/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /browse/i })).toBeInTheDocument();
  });

  it("renders children content", () => {
    render(AppShell, { props: { children: textSnippet("Test content here") } });
    expect(screen.getByText("Test content here")).toBeInTheDocument();
  });

  it("fires onnavigate with tab name when nav is clicked", async () => {
    const user = userEvent.setup();
    const handleNav = vi.fn();
    render(AppShell, {
      props: { children: textSnippet("content"), onnavigate: handleNav },
    });

    await user.click(screen.getByRole("button", { name: /party/i }));
    expect(handleNav).toHaveBeenCalledWith("queue");
  });

  it("highlights active tab", () => {
    const { container } = render(AppShell, {
      props: { children: textSnippet("content"), activeTab: "queue" },
    });
    const activeButton = container.querySelector("nav .active");
    expect(activeButton?.textContent).toContain("Party");
  });

  it("active class follows the activeTab prop", () => {
    // Re-rendering with different activeTab should move .active to the matching
    // nav button. The prior test only checked the queue case in isolation —
    // this catches a broken class:active binding that always highlights one tab.
    const { container: a } = render(AppShell, {
      props: { children: textSnippet("c"), activeTab: "browse" },
    });
    const browseActive = a.querySelector("nav .active");
    expect(browseActive?.textContent).toContain("Browse");
    cleanup();

    const { container: b } = render(AppShell, {
      props: { children: textSnippet("c"), activeTab: "playing" },
    });
    const playingActive = b.querySelector("nav .active");
    expect(playingActive?.textContent).toContain("Now Playing");
  });

  it("renders the banner snippet when provided", () => {
    render(AppShell, {
      props: { children: textSnippet("c"), banner: textSnippet("DECK OFFLINE") },
    });
    expect(screen.getByText("DECK OFFLINE")).toBeInTheDocument();
  });

  it("renders the headerEnd snippet when provided", () => {
    render(AppShell, {
      props: { children: textSnippet("c"), headerEnd: textSnippet("Alice · LEAVE") },
    });
    expect(screen.getByText("Alice · LEAVE")).toBeInTheDocument();
  });

  describe("Party tab badge", () => {
    it("renders the queue count on the Party tab", () => {
      const { container } = render(AppShell, {
        props: { children: textSnippet("c"), queueBadge: 3 },
      });
      const badge = container.querySelector(".nav-badge");
      expect(badge).not.toBeNull();
      expect(badge?.textContent).toBe("3");
    });

    it("hides the badge at zero and when undefined", () => {
      const { container: zero } = render(AppShell, {
        props: { children: textSnippet("c"), queueBadge: 0 },
      });
      expect(zero.querySelector(".nav-badge")).toBeNull();
      cleanup();

      const { container: absent } = render(AppShell, {
        props: { children: textSnippet("c") },
      });
      expect(absent.querySelector(".nav-badge")).toBeNull();
    });

    it("remounts the badge (bump animation) when the count increases", async () => {
      const { container, rerender } = render(AppShell, {
        props: { children: textSnippet("c"), queueBadge: 2 },
      });
      const before = container.querySelector(".nav-badge");
      await rerender({ queueBadge: 3 });
      const after = container.querySelector(".nav-badge");
      expect(after?.textContent).toBe("3");
      // {#key} remount replaces the DOM node — that's what replays the CSS bump.
      expect(after).not.toBe(before);
    });

    it("does not remount when the count decreases or stays equal", async () => {
      const { container, rerender } = render(AppShell, {
        props: { children: textSnippet("c"), queueBadge: 3 },
      });
      const before = container.querySelector(".nav-badge");
      await rerender({ queueBadge: 3 });
      const afterEqual = container.querySelector(".nav-badge");
      expect(afterEqual).toBe(before);
      await rerender({ queueBadge: 2 });
      const afterDecrease = container.querySelector(".nav-badge");
      expect(afterDecrease?.textContent).toBe("2");
      expect(afterDecrease).toBe(before);
    });
  });
});
