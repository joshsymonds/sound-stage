import { afterEach, describe, expect, it } from "vitest";

import { getGuestName, setGuestName } from "./session";

describe("session store", () => {
  afterEach(() => {
    document.cookie = "guest_name=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/";
  });

  it("returns null when no cookie is set", () => {
    expect(getGuestName()).toBeNull();
  });

  it("stores guest name in cookie", () => {
    setGuestName("Alice");
    expect(document.cookie).toContain("guest_name=Alice");
  });

  it("reads guest name from cookie", () => {
    document.cookie = "guest_name=Bob; path=/";
    expect(getGuestName()).toBe("Bob");
  });

  it("handles names with spaces", () => {
    setGuestName("DJ Sparkles");
    expect(getGuestName()).toBe("DJ Sparkles");
  });

  it("overwrites previous name", () => {
    setGuestName("Alice");
    setGuestName("Bob");
    expect(getGuestName()).toBe("Bob");
  });

  it("trims whitespace from names", () => {
    setGuestName("  Alice  ");
    expect(getGuestName()).toBe("Alice");
  });
});
