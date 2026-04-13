/**
 * Creates a text snippet for testing Svelte 5 components that accept `children: Snippet`.
 *
 * This uses Svelte's internal `$$anchor: Comment` pattern because @testing-library/svelte
 * doesn't have first-class snippet support for Svelte 5 yet. If this breaks on a Svelte
 * version bump, check the Svelte 5 compiled output for the current anchor pattern.
 */
export function textSnippet(text: string): unknown {
  return (($$anchor: Comment) => {
    const el = document.createElement("span");
    el.textContent = text;
    $$anchor.before(el);
  }) as unknown;
}
