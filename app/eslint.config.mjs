import js from "@eslint/js";
import reactHooks from "eslint-plugin-react-hooks";
import globals from "globals";
import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: ["node_modules", "android", "ios", ".expo"] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ["**/*.{ts,tsx}"],
    languageOptions: { ecmaVersion: 2022, globals: { ...globals.node, ...globals.jest } },
    plugins: { "react-hooks": reactHooks },
    rules: { ...reactHooks.configs.recommended.rules },
  },
  {
    // jest.mock factories run before imports, so the modules they build on are required inside them.
    files: ["**/*.test.{ts,tsx}", "jest.setup.ts"],
    rules: { "@typescript-eslint/no-require-imports": "off" },
  },
);
