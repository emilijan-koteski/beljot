import js from "@eslint/js";
import prettier from "eslint-config-prettier";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import simpleImportSort from "eslint-plugin-simple-import-sort";
import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: ["dist", "eslint.config.js"] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
      "simple-import-sort": simpleImportSort,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      "react-refresh/only-export-components": ["warn", { allowConstantExport: true }],
      "@typescript-eslint/no-unused-vars": ["error", { argsIgnorePattern: "^_" }],
      "simple-import-sort/imports": "error",
      "simple-import-sort/exports": "error",
      "no-restricted-syntax": [
        "error",
        {
          selector: "ExportDefaultDeclaration",
          message: "Named exports only — no export default. See project-context.md.",
        },
      ],
    },
  },
  prettier,
  {
    files: ["src/**/*.{ts,tsx}"],
    ignores: ["src/**/*.test.{ts,tsx}", "src/test-setup.ts"],
    rules: {
      "no-restricted-globals": [
        "error",
        ...[
          "process",
          "Buffer",
          "global",
          "__dirname",
          "__filename",
          "require",
          "module",
          "exports",
          "setImmediate",
          "clearImmediate",
        ].map((name) => ({ name, message: "Node-only global: not available in the browser." })),
      ],
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            { group: ["node:*"], message: "Node built-in: not available in the browser." },
          ],
        },
      ],
    },
  },
  {
    files: ["vite.config.ts", "vitest.config.ts"],
    rules: {
      "no-restricted-syntax": "off",
    },
  },
  {
    files: ["src/shared/components/ui/**/*.tsx"],
    rules: {
      "react-refresh/only-export-components": "off",
    },
  },
);
