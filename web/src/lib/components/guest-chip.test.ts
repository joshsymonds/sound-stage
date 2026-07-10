import { cleanup, render } from "@testing-library/svelte";
import { afterEach, describe, expect, it } from "vitest";

import GuestChip from "./GuestChip.svelte";

describe("GuestChip", () => {
  afterEach(cleanup);

  it("renders a single initial for a one-word name", () => {
    const { container } = render(GuestChip, { props: { name: "Alice" } });
    expect(container.querySelector(".guest-chip")?.textContent?.trim()).toBe(
      "A",
    );
  });

  it("renders two initials for a multi-word name", () => {
    const { container } = render(GuestChip, { props: { name: "Bob Marley" } });
    expect(container.querySelector(".guest-chip")?.textContent?.trim()).toBe(
      "BM",
    );
  });

  it("applies a non-empty background color", () => {
    const { container } = render(GuestChip, { props: { name: "Alice" } });
    const chip = container.querySelector<HTMLElement>(".guest-chip")!;
    expect(chip.style.background).not.toBe("");
  });

  it("applies different background colors for different names", () => {
    const first = render(GuestChip, { props: { name: "Alice" } });
    const firstColor =
      first.container.querySelector<HTMLElement>(".guest-chip")!.style
        .background;
    cleanup();
    const second = render(GuestChip, { props: { name: "Zelda" } });
    const secondColor =
      second.container.querySelector<HTMLElement>(".guest-chip")!.style
        .background;
    expect(firstColor).not.toBe(secondColor);
  });

  it("renders the same color for the same name across instances", () => {
    const first = render(GuestChip, { props: { name: "Charlie" } });
    const firstColor =
      first.container.querySelector<HTMLElement>(".guest-chip")!.style
        .background;
    cleanup();
    const second = render(GuestChip, { props: { name: "Charlie" } });
    const secondColor =
      second.container.querySelector<HTMLElement>(".guest-chip")!.style
        .background;
    expect(firstColor).toBe(secondColor);
  });

  it("does not render the name by default", () => {
    const { queryByText } = render(GuestChip, { props: { name: "Alice" } });
    expect(queryByText("Alice")).toBeNull();
  });

  it("renders the name when showName is true", () => {
    const { getByText } = render(GuestChip, {
      props: { name: "Alice", showName: true },
    });
    expect(getByText("Alice")).toBeInTheDocument();
  });
});
