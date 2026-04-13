import { cleanup, render, screen } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import Button from "./Button.svelte";

function textSnippet(text: string) {
  return (($$anchor: Comment) => {
    const el = document.createElement("span");
    el.textContent = text;
    $$anchor.before(el);
  }) as any;
}

describe("Button", () => {
  afterEach(cleanup);

  it("renders children text", () => {
    render(Button, {
      props: { children: textSnippet("Queue Song") },
    });
    expect(screen.getByRole("button", { name: "Queue Song" })).toBeInTheDocument();
  });

  it("applies primary variant by default", () => {
    render(Button, {
      props: { children: textSnippet("Click") },
    });
    const btn = screen.getByRole("button");
    expect(btn.className).toContain("primary");
  });

  it("applies secondary variant", () => {
    render(Button, {
      props: { variant: "secondary", children: textSnippet("Browse") },
    });
    const btn = screen.getByRole("button");
    expect(btn.className).toContain("secondary");
  });

  it("applies ghost variant", () => {
    render(Button, {
      props: { variant: "ghost", children: textSnippet("Cancel") },
    });
    const btn = screen.getByRole("button");
    expect(btn.className).toContain("ghost");
  });

  it("applies size classes", () => {
    render(Button, {
      props: { size: "sm", children: textSnippet("Small") },
    });
    expect(screen.getByRole("button").className).toContain("sm");
  });

  it("applies large size", () => {
    render(Button, {
      props: { size: "lg", children: textSnippet("Large") },
    });
    expect(screen.getByRole("button").className).toContain("lg");
  });

  it("handles click events", async () => {
    const user = userEvent.setup();
    const handleClick = vi.fn();
    render(Button, {
      props: { onclick: handleClick, children: textSnippet("Click me") },
    });

    await user.click(screen.getByRole("button"));
    expect(handleClick).toHaveBeenCalledOnce();
  });

  it("does not fire click when disabled", async () => {
    const user = userEvent.setup();
    const handleClick = vi.fn();
    render(Button, {
      props: { disabled: true, onclick: handleClick, children: textSnippet("Disabled") },
    });

    await user.click(screen.getByRole("button"));
    expect(handleClick).not.toHaveBeenCalled();
  });

  it("sets disabled attribute", () => {
    render(Button, {
      props: { disabled: true, children: textSnippet("Disabled") },
    });
    expect(screen.getByRole("button")).toBeDisabled();
  });
});
