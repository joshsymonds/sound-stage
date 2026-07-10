<script lang="ts">
  const ALL_LETTERS = [
    "#",
    "A",
    "B",
    "C",
    "D",
    "E",
    "F",
    "G",
    "H",
    "I",
    "J",
    "K",
    "L",
    "M",
    "N",
    "O",
    "P",
    "Q",
    "R",
    "S",
    "T",
    "U",
    "V",
    "W",
    "X",
    "Y",
    "Z",
  ];

  let { letters, onjump }: { letters: Set<string>; onjump: (letter: string) => void } = $props();

  let dragging = $state(false);

  function selectLetter(letter: string): void {
    if (letters.has(letter)) onjump(letter);
  }

  // The rail is one continuous pointer surface: pressing down starts a drag,
  // and every letter the pointer subsequently enters (without lifting) is
  // selected in turn — the standard iOS-style A-Z index gesture. Individual
  // 44px-tall targets aren't feasible for 27 letters, so letters rely on
  // native pointer hit-testing during the drag instead of per-letter
  // geometry math.
  function handlePointerDown(letter: string): void {
    dragging = true;
    selectLetter(letter);
  }

  function handlePointerEnter(letter: string): void {
    if (dragging) selectLetter(letter);
  }

  function endDrag(): void {
    dragging = false;
  }

  $effect(() => {
    globalThis.addEventListener("pointerup", endDrag);
    globalThis.addEventListener("pointercancel", endDrag);
    return () => {
      globalThis.removeEventListener("pointerup", endDrag);
      globalThis.removeEventListener("pointercancel", endDrag);
    };
  });
</script>

<div class="rail" role="listbox" aria-label="Jump to letter" aria-orientation="vertical">
  {#each ALL_LETTERS as letter (letter)}
    {@const present = letters.has(letter)}
    <span
      class="letter"
      class:dim={!present}
      role="option"
      aria-selected="false"
      aria-disabled={present ? undefined : "true"}
      tabindex="-1"
      onpointerdown={() => handlePointerDown(letter)}
      onpointerenter={() => handlePointerEnter(letter)}
    >
      {letter}
    </span>
  {/each}
</div>

<style>
  .rail {
    position: fixed;
    right: 0;
    top: 50%;
    transform: translateY(-50%);
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: var(--space-xs) 2px;
    touch-action: none;
    user-select: none;
  }

  .letter {
    font-size: 0.5625rem;
    font-weight: 600;
    line-height: 1.4;
    color: var(--color-text-dim);
    cursor: pointer;
  }

  .letter.dim {
    color: var(--color-text-muted);
    opacity: 0.4;
    cursor: default;
  }
</style>
