import { dirname } from "node:path";
import { fileURLToPath } from "node:url";

import eslint from "@eslint/js";
import prettier from "eslint-config-prettier";
import perfectionist from "eslint-plugin-perfectionist";
import sonarjs from "eslint-plugin-sonarjs";
import svelte from "eslint-plugin-svelte";
import unicorn from "eslint-plugin-unicorn";
import vitest from "@vitest/eslint-plugin";
import globals from "globals";
import tseslint from "typescript-eslint";
import svelteParser from "svelte-eslint-parser";

const tsconfigRootDir = dirname(fileURLToPath(import.meta.url));

export default tseslint.config(
  // ── Ignores ──────────────────────────────────────────────
  {
    ignores: [
      ".svelte-kit/**",
      ".storybook/**",
      "build/**",
      "node_modules/**",
      "storybook-static/**",
      "*.config.ts",
      "*.config.js",
      "vitest-setup.ts",
      "scripts/**",
    ],
  },

  // ── Base presets ──────────────────────────────────────────
  eslint.configs.recommended,
  tseslint.configs.strictTypeChecked,
  tseslint.configs.stylisticTypeChecked,
  unicorn.configs.recommended,
  ...(sonarjs.configs.recommended ? [sonarjs.configs.recommended].flat() : []),
  ...svelte.configs.recommended,
  prettier,

  // ── Parser settings ──────────────────────────────────────
  {
    languageOptions: {
      globals: globals.browser,
      parserOptions: {
        projectService: {
          allowDefaultProject: ["scripts/*.ts"],
        },
        tsconfigRootDir,
        extraFileExtensions: [".svelte"],
      },
    },
  },

  // ── Svelte files ─────────────────────────────────────────
  {
    files: ["**/*.svelte", "**/*.svelte.ts"],
    languageOptions: {
      parser: svelteParser,
      parserOptions: {
        parser: tseslint.parser,
      },
    },
  },

  // ── All TypeScript + Svelte files ─────────────────────────
  {
    files: ["src/**/*.ts", "src/**/*.svelte"],
    plugins: { perfectionist },
    rules: {
      // ── Correctness ──────────────────────────────────────
      "@typescript-eslint/no-floating-promises": "error",
      "@typescript-eslint/no-misused-promises": "error",
      "@typescript-eslint/switch-exhaustiveness-check": "error",

      // ── Type safety ──────────────────────────────────────
      "@typescript-eslint/no-explicit-any": "error",
      "@typescript-eslint/no-non-null-assertion": "error",
      "@typescript-eslint/consistent-type-imports": [
        "error",
        { prefer: "type-imports", fixStyle: "separate-type-imports" },
      ],
      "@typescript-eslint/explicit-function-return-type": [
        "error",
        {
          allowExpressions: true,
          allowTypedFunctionExpressions: true,
          allowHigherOrderFunctions: true,
          allowDirectConstAssertionInArrowFunctions: true,
        },
      ],

      // ── Unused code ──────────────────────────────────────
      "@typescript-eslint/no-unused-vars": [
        "error",
        {
          args: "all",
          argsIgnorePattern: "^_",
          varsIgnorePattern: "^_",
          caughtErrors: "all",
          caughtErrorsIgnorePattern: "^_",
        },
      ],

      // ── Complexity ────────────────────────────────────────
      "sonarjs/cognitive-complexity": ["error", 15],
      complexity: ["error", 15],
      "max-depth": ["error", 4],
      "max-lines-per-function": ["error", { max: 80, skipBlankLines: true, skipComments: true }],
      "max-nested-callbacks": ["error", 3],

      // ── Naming ────────────────────────────────────────────
      "@typescript-eslint/naming-convention": [
        "error",
        { selector: "default", format: ["camelCase"] },
        {
          selector: "variable",
          format: ["camelCase", "UPPER_CASE"],
        },
        {
          selector: "variable",
          modifiers: ["const", "exported"],
          format: ["camelCase", "PascalCase", "UPPER_CASE"],
        },
        {
          selector: "parameter",
          format: ["camelCase"],
          leadingUnderscore: "allow",
        },
        { selector: "typeLike", format: ["PascalCase"] },
        { selector: "enumMember", format: ["PascalCase"] },
        { selector: "property", format: null },
        { selector: "import", format: null },
      ],

      // ── No print statements ───────────────────────────────
      "no-console": "error",

      // ── Comment hygiene ───────────────────────────────────
      "no-warning-comments": ["warn", { terms: ["fixme", "hack", "xxx", "bug"] }],
      "sonarjs/todo-tag": "warn",

      // ── Import organization ───────────────────────────────
      "perfectionist/sort-imports": [
        "error",
        {
          type: "natural",
          groups: [
            "builtin",
            { newlinesBetween: 1 },
            "external",
            { newlinesBetween: 1 },
            "internal",
            "parent",
            "sibling",
            "index",
          ],
        },
      ],
      "perfectionist/sort-named-imports": ["error", { type: "natural" }],
      "perfectionist/sort-exports": ["error", { type: "natural" }],

      // ── General quality ───────────────────────────────────
      eqeqeq: ["error", "always"],
      "no-eval": "error",
      "no-implied-eval": "error",
      "prefer-const": "error",
      "no-var": "error",
      "object-shorthand": "error",
      "prefer-template": "error",

      // ── Unicorn overrides ─────────────────────────────────
      "unicorn/no-null": "off",
      "unicorn/prevent-abbreviations": [
        "error",
        {
          replacements: {
            args: false,
            config: false,
            ctx: false,
            db: false,
            el: false,
            env: false,
            err: false,
            fn: false,
            msg: false,
            params: false,
            props: false,
            ref: false,
            req: false,
            res: false,
            ws: false,
          },
        },
      ],
      "unicorn/catch-error-name": ["error", { ignore: ["^err$"] }],
      "unicorn/filename-case": ["error", { case: "kebabCase" }],
    },
  },

  // ── Svelte-specific relaxations ───────────────────────────
  {
    files: ["**/*.svelte"],
    rules: {
      "unicorn/filename-case": "off",
      "@typescript-eslint/explicit-function-return-type": "off",
      "max-lines-per-function": "off",
      "prefer-const": "off",
      "@typescript-eslint/no-unsafe-assignment": "off",
      "@typescript-eslint/no-unsafe-call": "off",
      "@typescript-eslint/no-unsafe-member-access": "off",
      "@typescript-eslint/no-unsafe-argument": "off",
      "@typescript-eslint/no-confusing-void-expression": "off",
      "@typescript-eslint/prefer-nullish-coalescing": "off",
      "sonarjs/no-use-of-empty-return-value": "off",
    },
  },

  // ── Test-specific relaxations ─────────────────────────────
  {
    files: ["src/**/*.test.ts"],
    plugins: { vitest },
    rules: {
      ...vitest.configs.recommended.rules,

      "@typescript-eslint/no-floating-promises": "off",
      "@typescript-eslint/no-unsafe-assignment": "off",
      "@typescript-eslint/no-unsafe-member-access": "off",
      "@typescript-eslint/no-unsafe-call": "off",
      "@typescript-eslint/no-unsafe-return": "off",
      "@typescript-eslint/no-unsafe-argument": "off",
      "@typescript-eslint/no-explicit-any": "off",
      "@typescript-eslint/no-non-null-assertion": "off",
      "@typescript-eslint/consistent-type-assertions": "off",
      "@typescript-eslint/explicit-function-return-type": "off",
      "@typescript-eslint/naming-convention": "off",
      "sonarjs/cognitive-complexity": "off",
      "sonarjs/no-duplicate-string": "off",
      "max-lines-per-function": "off",
      "max-nested-callbacks": "off",
      "unicorn/consistent-function-scoping": "off",
      "unicorn/no-document-cookie": "off",
      complexity: "off",
      "no-console": "off",
    },
  },

  // ── Storybook stories relaxations ─────────────────────────
  {
    files: ["src/**/*.stories.ts", "src/**/*.stories.svelte"],
    rules: {
      "@typescript-eslint/explicit-function-return-type": "off",
      "@typescript-eslint/naming-convention": "off",
      "@typescript-eslint/no-empty-function": "off",
      "@typescript-eslint/no-unsafe-assignment": "off",
      "max-lines-per-function": "off",
      "no-console": "off",
      "sonarjs/no-globals-shadowing": "off",
    },
  },
);
