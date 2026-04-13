<script lang="ts">
  import Button from "./Button.svelte";

  let {
    onsubmit,
  }: {
    onsubmit: (name: string) => void;
  } = $props();

  let name = $state("");

  function handleSubmit(): void {
    const trimmed = name.trim();
    if (trimmed.length > 0) {
      onsubmit(trimmed);
    }
  }

  function handleKeydown(event: KeyboardEvent): void {
    if (event.key === "Enter") {
      handleSubmit();
    }
  }
</script>

<div class="name-entry">
  <div class="content">
    <h1 class="logo">SoundStage</h1>
    <p class="subtitle">Karaoke night awaits</p>

    <div class="form">
      <input
        type="text"
        class="input"
        placeholder="Your name"
        bind:value={name}
        onkeydown={handleKeydown}
        maxlength={24}
      />
      <Button size="lg" onclick={handleSubmit}>Join</Button>
    </div>
  </div>
</div>

<style>
  .name-entry {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    min-height: 100dvh;
    padding: var(--space-lg);
    background: var(--color-bg);
  }

  .content {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-lg);
    width: 100%;
    max-width: 320px;
  }

  .logo {
    font-size: 2.5rem;
    font-weight: 800;
    color: var(--color-pink);
    text-shadow: var(--glow-text-pink);
    letter-spacing: -0.02em;
  }

  .subtitle {
    font-size: 1rem;
    color: var(--color-text-muted);
    margin-top: calc(-1 * var(--space-md));
  }

  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
    width: 100%;
  }

  .input {
    width: 100%;
    padding: 14px 16px;
    background: var(--color-surface);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-md);
    color: var(--color-text);
    font-family: var(--font-body);
    font-size: 1rem;
    text-align: center;
    outline: none;
    transition: border-color var(--transition-normal), box-shadow var(--transition-normal);
  }

  .input:focus {
    border-color: var(--color-pink);
    box-shadow: var(--glow-pink);
  }

  .input::placeholder {
    color: var(--color-text-muted);
  }
</style>
