import js from "@eslint/js";
import ts from "typescript-eslint";
import svelte from "eslint-plugin-svelte";
import svelteParser from "svelte-eslint-parser";
import prettier from "eslint-config-prettier";
import globals from "globals";

export default ts.config(
  js.configs.recommended,
  ...ts.configs.strictTypeChecked,
  ...ts.configs.stylisticTypeChecked,
  ...svelte.configs["flat/recommended"],
  prettier,
  ...svelte.configs["flat/prettier"],
  {
    languageOptions: {
      ecmaVersion: 2023,
      sourceType: "module",
      globals: { ...globals.browser, ...globals.node },
      parserOptions: {
        project: ["./tsconfig.app.json", "./tsconfig.node.json"],
        tsconfigRootDir: import.meta.dirname,
        extraFileExtensions: [".svelte"],
      },
    },
  },
  {
    files: ["**/*.svelte"],
    languageOptions: {
      parser: svelteParser,
      parserOptions: { parser: ts.parser },
    },
  },
  {
    // eslint-plugin-svelte's own base config matches *.svelte.ts/*.svelte.js
    // (the Svelte 5 "runes module" convention) and hands them to
    // svelte-eslint-parser, but without delegating to the TS parser the way
    // it does for *.svelte files above -- so a plain `import type { X }`
    // fails to parse. Same fix as the .svelte override, for the same reason.
    files: ["**/*.svelte.ts", "**/*.svelte.js"],
    languageOptions: {
      parser: svelteParser,
      parserOptions: { parser: ts.parser },
    },
  },
  {
    ignores: [
      "dist/",
      "node_modules/",
      ".svelte-kit/",
      "*.config.js",
      "*.config.ts",
    ],
  },
);
