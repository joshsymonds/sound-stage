import { readFileSync } from "node:fs";
import path from "node:path";

import { cleanup, render } from "@testing-library/svelte";
import { tick } from "svelte";
import { afterEach, describe, expect, it } from "vitest";

import MarqueeTestHarness from "./MarqueeTestHarness.svelte";

// Vitest disables CSS processing for this project (no `test.css` option in
// vite.config.ts), so Svelte's compiled <style> block is never injected into
// the jsdom document during tests: getComputedStyle cannot observe it for any
// component here. These tests instead assert on the generated CSS source
// itself, extracting each rule's real body rather than matching a substring.
// (import.meta.url can't locate the file here either: this project's
// `resolve.conditions: ["browser"]` for Vitest makes it resolve against
// jsdom's fake location instead of the real file path, so we use
// process.cwd(), which is unaffected by that module-resolution condition.)
function extractBraceBlock(source: string, openBraceIndex: number): string {
  let depth = 0;
  for (let index = openBraceIndex; index < source.length; index++) {
    if (source[index] === "{") depth++;
    if (source[index] === "}") {
      depth--;
      if (depth === 0) return source.slice(openBraceIndex + 1, index);
    }
  }
  throw new Error("Unbalanced braces in CSS source");
}

function extractRuleBody(css: string, selector: string): string {
  const escapedSelector = selector.replaceAll(/[.()]/g, String.raw`\$&`);
  const pattern = new RegExp(`(?:^|\\n|\\})\\s*${escapedSelector}\\s*\\{`);
  const match = pattern.exec(css);
  if (!match) {
    throw new Error(`Could not find rule for selector "${selector}"`);
  }
  const openBraceIndex = match.index + match[0].length - 1;
  return extractBraceBlock(css, openBraceIndex);
}

const marqueeSource = readFileSync(
  path.resolve(process.cwd(), "src/lib/components/Marquee.svelte"),
  "utf8",
);
const styleBlock = /<style>([\s\S]*)<\/style>/.exec(marqueeSource)?.[1] ?? "";
const reducedMotionBlock = extractRuleBody(
  styleBlock,
  "@media (prefers-reduced-motion: reduce)",
);

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

  it("does not apply a mask-image in the base (non-scrolling) track rule", () => {
    const baseBody = extractRuleBody(styleBlock, ".track");
    expect(baseBody).toMatch(/mask-image:\s*none/);
    expect(baseBody).toMatch(/-webkit-mask-image:\s*none/);
  });

  it("applies a fading mask-image to the clip edge only in the scrolling track rule", () => {
    const scrollingBody = extractRuleBody(styleBlock, ".track.scrolling");
    expect(scrollingBody).toMatch(/mask-image:\s*linear-gradient\(/);
    expect(scrollingBody).toMatch(/-webkit-mask-image:\s*linear-gradient\(/);
  });

  it("removes the mask-image in the reduced-motion single-segment fallback", () => {
    const scrollingBody = extractRuleBody(reducedMotionBlock, ".track.scrolling");
    expect(scrollingBody).toMatch(/mask-image:\s*none/);
    expect(scrollingBody).toMatch(/-webkit-mask-image:\s*none/);
  });
});
