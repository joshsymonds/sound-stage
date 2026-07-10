import { cleanup, render } from "@testing-library/svelte";
import { afterEach, describe, expect, it } from "vitest";

import EqualizerGlyph from "./EqualizerGlyph.svelte";

describe("EqualizerGlyph", () => {
  afterEach(cleanup);

  it("adds the animating class when playing", () => {
    const { container } = render(EqualizerGlyph, { props: { playing: true } });
    const glyph = container.querySelector(".equalizer");
    expect(glyph).toBeInTheDocument();
    expect(glyph).toHaveClass("playing");
  });

  it("omits the animating class when not playing", () => {
    const { container } = render(EqualizerGlyph, { props: { playing: false } });
    const glyph = container.querySelector(".equalizer");
    expect(glyph).toBeInTheDocument();
    expect(glyph).not.toHaveClass("playing");
  });

  it("renders three bars", () => {
    const { container } = render(EqualizerGlyph, { props: { playing: true } });
    expect(container.querySelectorAll(".bar")).toHaveLength(3);
  });
});
