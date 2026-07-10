import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";

import FastScrollRail from "./FastScrollRail.svelte";

describe("FastScrollRail", () => {
  afterEach(cleanup);

  it("renders all 27 letters (# and A-Z)", () => {
    render(FastScrollRail, { props: { letters: new Set(["A"]), onjump: vi.fn() } });
    expect(screen.getByText("#")).toBeInTheDocument();
    for (const letter of "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
      expect(screen.getByText(letter)).toBeInTheDocument();
    }
  });

  it("dims letters that are absent from the set", () => {
    render(FastScrollRail, { props: { letters: new Set(["A", "B"]), onjump: vi.fn() } });
    expect(screen.getByText("A")).not.toHaveClass("dim");
    expect(screen.getByText("C")).toHaveClass("dim");
  });

  it("marks absent letters as inert for assistive tech", () => {
    render(FastScrollRail, { props: { letters: new Set(["A"]), onjump: vi.fn() } });
    expect(screen.getByText("A")).not.toHaveAttribute("aria-disabled", "true");
    expect(screen.getByText("B")).toHaveAttribute("aria-disabled", "true");
  });

  it("fires onjump when a present letter is pressed", async () => {
    const handleJump = vi.fn();
    render(FastScrollRail, { props: { letters: new Set(["Q"]), onjump: handleJump } });

    await fireEvent.pointerDown(screen.getByText("Q"));
    expect(handleJump).toHaveBeenCalledWith("Q");
  });

  it("does not fire onjump when an absent letter is pressed", async () => {
    const handleJump = vi.fn();
    render(FastScrollRail, { props: { letters: new Set(["Q"]), onjump: handleJump } });

    await fireEvent.pointerDown(screen.getByText("Z"));
    expect(handleJump).not.toHaveBeenCalled();
  });

  it("selects each present letter dragged over without lifting the pointer", async () => {
    const handleJump = vi.fn();
    render(FastScrollRail, { props: { letters: new Set(["A", "B", "C"]), onjump: handleJump } });

    await fireEvent.pointerDown(screen.getByText("A"));
    await fireEvent.pointerEnter(screen.getByText("B"));
    await fireEvent.pointerEnter(screen.getByText("C"));

    expect(handleJump.mock.calls).toEqual([["A"], ["B"], ["C"]]);
  });

  it("stops selecting letters after the pointer is released", async () => {
    const handleJump = vi.fn();
    render(FastScrollRail, { props: { letters: new Set(["A", "B"]), onjump: handleJump } });

    await fireEvent.pointerDown(screen.getByText("A"));
    await fireEvent.pointerUp(screen.getByText("A"));
    handleJump.mockClear();

    await fireEvent.pointerEnter(screen.getByText("B"));
    expect(handleJump).not.toHaveBeenCalled();
  });

  it("does not select absent letters while dragging", async () => {
    const handleJump = vi.fn();
    render(FastScrollRail, { props: { letters: new Set(["A", "C"]), onjump: handleJump } });

    await fireEvent.pointerDown(screen.getByText("A"));
    await fireEvent.pointerEnter(screen.getByText("B"));
    await fireEvent.pointerEnter(screen.getByText("C"));

    expect(handleJump.mock.calls).toEqual([["A"], ["C"]]);
  });
});
