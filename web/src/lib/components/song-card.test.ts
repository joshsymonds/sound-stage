import { cleanup, render, screen } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import SongCard from "./SongCard.svelte";

describe("SongCard", () => {
  afterEach(cleanup);

  it("renders title and artist", () => {
    render(SongCard, {
      props: { title: "Bohemian Rhapsody", artist: "Queen" },
    });
    expect(screen.getByText("Bohemian Rhapsody")).toBeInTheDocument();
    expect(screen.getByText("Queen")).toBeInTheDocument();
  });

  it("renders edition when provided", () => {
    render(SongCard, {
      props: { title: "Waterloo", artist: "ABBA", edition: "ESC 1974" },
    });
    expect(screen.getByText("ESC 1974")).toBeInTheDocument();
  });

  it("renders year when provided", () => {
    render(SongCard, {
      props: { title: "Waterloo", artist: "ABBA", year: 1974 },
    });
    expect(screen.getByText("1974")).toBeInTheDocument();
  });

  it("renders edition and year together", () => {
    render(SongCard, {
      props: { title: "Waterloo", artist: "ABBA", edition: "ESC 1974", year: 1974 },
    });
    const meta = screen.getByText(/ESC 1974/);
    expect(meta.textContent).toContain("1974");
  });

  it("shows placeholder when no cover URL", () => {
    const { container } = render(SongCard, {
      props: { title: "Test", artist: "Test" },
    });
    expect(container.querySelector(".cover-placeholder")).toBeInTheDocument();
  });

  it("shows cover image when URL provided", () => {
    render(SongCard, {
      props: { title: "Test", artist: "Test", coverUrl: "/cover.jpg" },
    });
    const img = screen.getByAltText("Test — Test");
    expect(img).toBeInTheDocument();
    expect(img).toHaveAttribute("src", "/cover.jpg");
  });

  it("is clickable when onclick provided", async () => {
    const user = userEvent.setup();
    const handleClick = vi.fn();
    render(SongCard, {
      props: { title: "Test", artist: "Test", onclick: handleClick },
    });

    await user.click(screen.getByRole("button"));
    expect(handleClick).toHaveBeenCalledOnce();
  });

  it("is not interactive without onclick", () => {
    render(SongCard, {
      props: { title: "Test", artist: "Test" },
    });
    expect(screen.getByRole("button")).toBeDisabled();
  });

  it("renders a badge when provided", () => {
    render(SongCard, {
      props: { title: "Test", artist: "Test", badge: "instant" },
    });
    expect(screen.getByText("instant")).toBeInTheDocument();
  });

  it("does not render a badge when omitted", () => {
    const { container } = render(SongCard, {
      props: { title: "Test", artist: "Test" },
    });
    expect(container.querySelector(".badge")).toBeNull();
  });
});
