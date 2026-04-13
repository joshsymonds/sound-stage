import type { Preview } from "@storybook/svelte";

import "../src/app.css";

const preview: Preview = {
  parameters: {
    layout: "centered",
    backgrounds: {
      default: "soundstage",
      values: [{ name: "soundstage", value: "#0a0a0f" }],
    },
  },
};

export default preview;
