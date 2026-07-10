import { cleanup, render, screen } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import { tick } from "svelte";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { Song } from "../types";

import CoverTile from "./CoverTile.svelte";

const song: Song = { id: "42", title: "Bohemian Rhapsody", artist: "Queen" };

describe("CoverTile", () => {
  afterEach(cleanup);

  it("renders title and artist", () => {
    render(CoverTile, { props: { song, onqueue: vi.fn() } });
    expect(screen.getByText("Bohemian Rhapsody")).toBeInTheDocument();
    expect(screen.getByText("Queen")).toBeInTheDocument();
  });

  it("renders the thumbnail URL", () => {
    const { container } = render(CoverTile, { props: { song, onqueue: vi.fn() } });
    const img = container.querySelector("img");
    expect(img).toHaveAttribute("src", "/api/library/42/thumb");
  });

  it("fires onqueue with the song when clicked", async () => {
    const user = userEvent.setup();
    const handleQueue = vi.fn();
    render(CoverTile, { props: { song, onqueue: handleQueue } });

    await user.click(screen.getByRole("button"));
    expect(handleQueue).toHaveBeenCalledWith(song);
  });

  it("falls back to initials when the thumbnail fails to load", async () => {
    const { container } = render(CoverTile, { props: { song, onqueue: vi.fn() } });

    const img = container.querySelector("img");
    img?.dispatchEvent(new Event("error"));
    await tick();

    expect(container.querySelector(".fallback")).toBeInTheDocument();
    expect(screen.getByText("Q")).toBeInTheDocument();
  });

  it("uses up to two words for the fallback initials", async () => {
    const { container } = render(CoverTile, {
      props: { song: { id: "1", title: "T", artist: "Tears for Fears" }, onqueue: vi.fn() },
    });
    const img = container.querySelector("img");
    img?.dispatchEvent(new Event("error"));
    await tick();
    expect(screen.getByText("TF")).toBeInTheDocument();
  });

  it("falls back to initials when the song has no id", () => {
    const { container } = render(CoverTile, {
      props: { song: { id: "", title: "T", artist: "Queen" }, onqueue: vi.fn() },
    });
    expect(container.querySelector(".fallback")).toBeInTheDocument();
    expect(screen.getByText("Q")).toBeInTheDocument();
  });

  it("applies an optional DOM id to the root button, for scroll anchoring", () => {
    render(CoverTile, { props: { song, onqueue: vi.fn(), id: "crate-Q" } });
    expect(screen.getByRole("button")).toHaveAttribute("id", "crate-Q");
  });

  it("omits the DOM id when none is given", () => {
    render(CoverTile, { props: { song, onqueue: vi.fn() } });
    expect(screen.getByRole("button")).not.toHaveAttribute("id");
  });
});
