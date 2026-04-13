import { cleanup, render, screen } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import NameEntry from "./NameEntry.svelte";

describe("NameEntry", () => {
  afterEach(cleanup);

  it("renders heading and input", () => {
    render(NameEntry, { props: { onsubmit: vi.fn() } });
    expect(screen.getByText("SoundStage")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Your name")).toBeInTheDocument();
  });

  it("renders a submit button", () => {
    render(NameEntry, { props: { onsubmit: vi.fn() } });
    expect(screen.getByRole("button", { name: /join/i })).toBeInTheDocument();
  });

  it("calls onsubmit with trimmed name", async () => {
    const user = userEvent.setup();
    const handleSubmit = vi.fn();
    render(NameEntry, { props: { onsubmit: handleSubmit } });

    await user.type(screen.getByPlaceholderText("Your name"), "  Alice  ");
    await user.click(screen.getByRole("button", { name: /join/i }));

    expect(handleSubmit).toHaveBeenCalledWith("Alice");
  });

  it("does not submit empty name", async () => {
    const user = userEvent.setup();
    const handleSubmit = vi.fn();
    render(NameEntry, { props: { onsubmit: handleSubmit } });

    await user.click(screen.getByRole("button", { name: /join/i }));

    expect(handleSubmit).not.toHaveBeenCalled();
  });

  it("does not submit whitespace-only name", async () => {
    const user = userEvent.setup();
    const handleSubmit = vi.fn();
    render(NameEntry, { props: { onsubmit: handleSubmit } });

    await user.type(screen.getByPlaceholderText("Your name"), "   ");
    await user.click(screen.getByRole("button", { name: /join/i }));

    expect(handleSubmit).not.toHaveBeenCalled();
  });

  it("submits on Enter key", async () => {
    const user = userEvent.setup();
    const handleSubmit = vi.fn();
    render(NameEntry, { props: { onsubmit: handleSubmit } });

    await user.type(screen.getByPlaceholderText("Your name"), "Bob{Enter}");

    expect(handleSubmit).toHaveBeenCalledWith("Bob");
  });
});
