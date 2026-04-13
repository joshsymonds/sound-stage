<script lang="ts">
  import type { Snippet } from "svelte";
  import type { HTMLButtonAttributes } from "svelte/elements";

  let {
    variant = "primary",
    size = "default",
    children,
    ...rest
  }: HTMLButtonAttributes & {
    variant?: "primary" | "secondary" | "ghost";
    size?: "sm" | "default" | "lg";
    children: Snippet;
  } = $props();
</script>

<button class="btn {variant} {size}" {...rest}>
  {@render children()}
</button>

<style>
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-sm);
    border-radius: var(--radius-md);
    font-family: var(--font-body);
    font-weight: 600;
    cursor: pointer;
    transition: all var(--transition-normal);
    border: none;
    outline: none;
    line-height: 1;
  }

  .btn:focus-visible {
    outline: 2px solid var(--color-pink);
    outline-offset: 2px;
  }

  /* ── Variants ─────────────────────────────────────────── */

  .primary {
    background: var(--color-pink);
    color: white;
    box-shadow: var(--glow-pink);
  }

  .primary:hover:not(:disabled) {
    background: var(--color-pink-light);
    box-shadow: var(--glow-pink-strong);
  }

  .primary:disabled {
    background: var(--color-pink-dim);
    opacity: 0.5;
    cursor: not-allowed;
    box-shadow: none;
  }

  .secondary {
    background: var(--color-surface-raised);
    color: var(--color-text);
    border: 1px solid var(--color-border);
  }

  .secondary:hover:not(:disabled) {
    background: var(--color-surface);
    border-color: var(--color-pink);
    box-shadow: var(--glow-pink);
  }

  .secondary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .ghost {
    background: transparent;
    color: var(--color-text-dim);
  }

  .ghost:hover:not(:disabled) {
    color: var(--color-text);
    background: rgba(255, 255, 255, 0.04);
  }

  .ghost:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  /* ── Sizes ────────────────────────────────────────────── */

  .sm {
    padding: 6px 14px;
    font-size: 0.8125rem;
  }

  .default {
    padding: 10px 20px;
    font-size: 0.875rem;
  }

  .lg {
    padding: 14px 28px;
    font-size: 1rem;
  }
</style>
