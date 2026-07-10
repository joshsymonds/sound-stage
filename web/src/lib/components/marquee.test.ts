import { cleanup, render } from "@testing-library/svelte";
import { tick } from "svelte";
import { afterEach, describe, expect, it } from "vitest";

import MarqueeTestHarness from "./MarqueeTestHarness.svelte";

const originalScrollWidth = Object.getOwnPropertyDescriptor(HTMLElement.prototype, "scrollWidth");
const originalClientWidth = Object.getOwnPropertyDescriptor(HTMLElement.prototype, "clientWidth");

function mockWidths(scrollWidth: number, clientWidth: number): void {
  Object.defineProperty(HTMLElement.prototype, "scrollWidth", {
    configurable: true,
    get() {
      return scrollWidth;
    },
  });
  Object.defineProperty(HTMLElement.prototype, "clientWidth", {
    configurable: true,
    get() {
      return clientWidth;
    },
  });
}

function restoreWidths(): void {
  if (originalScrollWidth) {
    Object.defineProperty(HTMLElement.prototype, "scrollWidth", originalScrollWidth);
  }
  if (originalClientWidth) {
    Object.defineProperty(HTMLElement.prototype, "clientWidth", originalClientWidth);
  }
}

describe("Marquee", () => {
  afterEach(() => {
    cleanup();
    restoreWidths();
  });

  it("renders the slotted content", async () => {
    mockWidths(100, 100);
    const { getByText } = render(MarqueeTestHarness, { props: { text: "Bohemian Rhapsody" } });
    await tick();
    expect(getByText("Bohemian Rhapsody")).toBeInTheDocument();
  });

  it("does not add the scrolling class when content fits its container", async () => {
    mockWidths(100, 100);
    const { container } = render(MarqueeTestHarness);
    await tick();
    const track = container.querySelector(".track");
    expect(track).toBeInTheDocument();
    expect(track).not.toHaveClass("scrolling");
  });

  it("adds the scrolling class when content overflows its container", async () => {
    mockWidths(500, 100);
    const { container } = render(MarqueeTestHarness);
    await tick();
    const track = container.querySelector(".track");
    expect(track).toHaveClass("scrolling");
  });

  it("renders a second, hidden copy of the content only while scrolling", async () => {
    mockWidths(500, 100);
    const { container } = render(MarqueeTestHarness, { props: { text: "Dancing Queen" } });
    await tick();
    expect(container.querySelectorAll(".segment")).toHaveLength(2);
  });
});
