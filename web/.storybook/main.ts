import type { StorybookConfig } from "@storybook/sveltekit";

const config: StorybookConfig = {
  stories: ["../src/**/*.stories.@(ts|svelte)"],
  staticDirs: ["../static"],
  addons: ["@storybook/addon-svelte-csf"],
  framework: "@storybook/sveltekit",
};

export default config;
