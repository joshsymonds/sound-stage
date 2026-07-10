<script lang="ts">
  import Marquee from "./Marquee.svelte";

  let { text = "Sample Text" }: { text?: string } = $props();
</script>

<Marquee>{text}</Marquee>
