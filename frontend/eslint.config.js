import path from "node:path"
import { fileURLToPath } from "node:url"
import { defineConfig } from "eslint/config"
import js from "@eslint/js"
import importPlugin from "eslint-plugin-import"
import jsxA11y from "eslint-plugin-jsx-a11y"
import globals from "globals"
import reactHooks from "eslint-plugin-react-hooks"
import reactRefresh from "eslint-plugin-react-refresh"
import security from "eslint-plugin-security"
import tseslint from "typescript-eslint"

const __dirname = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig(
  {
    ignores: ["dist", "coverage"]
  },
  {
    files: ["**/*.{ts,tsx}"],
    extends: [
      js.configs.recommended,
      ...tseslint.configs.recommended,
      ...tseslint.configs.recommendedTypeChecked,
      importPlugin.flatConfigs.recommended,
      importPlugin.flatConfigs.typescript,
      jsxA11y.flatConfigs.recommended
    ],
    languageOptions: {
      ecmaVersion: 2020,
      sourceType: "module",
      globals: {
        ...globals.browser
      },
      parserOptions: {
        projectService: true,
        tsconfigRootDir: __dirname
      }
    },
    settings: {
      "import/resolver": {
        typescript: {
          alwaysTryTypes: true,
          project: "./tsconfig.json"
        },
        node: {
          extensions: [".js", ".jsx", ".ts", ".tsx"]
        }
      }
    },
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
      security
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      "react-refresh/only-export-components": ["warn", { allowConstantExport: true }],
      "import/no-cycle": ["error", { maxDepth: 5, ignoreExternal: true }],
      "import/no-duplicates": "error",
      "import/no-self-import": "error",
      "import/no-unresolved": ["error", { ignore: ["\\.(css)$"] }],
      "import/no-useless-path-segments": ["error", { noUselessIndex: true }],
      complexity: ["error", 20],
      "max-lines-per-function": ["error", { max: 300, skipBlankLines: true, skipComments: true }],
      "max-depth": ["error", 4],
      "max-params": ["error", 5],
      "security/detect-bidi-characters": "error",
      "security/detect-eval-with-expression": "error",
      "security/detect-new-buffer": "error",
      "security/detect-non-literal-regexp": "error",
      "security/detect-unsafe-regex": "error"
    }
  },
  {
    files: ["src/App.tsx"],
    rules: {
      // TODO(plato-techdebt-app-001, target 2026-06-30): Split App.tsx into smaller components and remove this temporary exception.
      complexity: ["error", 45],
      "max-lines-per-function": "off"
    }
  },
  {
    files: ["**/*.{test,spec}.{ts,tsx}", "src/setupTests.ts", "src/test-utils/**/*.ts"],
    languageOptions: {
      globals: {
        ...globals.vitest
      }
    },
    rules: {
      "@typescript-eslint/no-base-to-string": "off",
      "@typescript-eslint/no-misused-promises": "off",
      "@typescript-eslint/no-unnecessary-type-assertion": "off",
      "@typescript-eslint/no-unsafe-argument": "off",
      "@typescript-eslint/no-unsafe-assignment": "off",
      "@typescript-eslint/no-unsafe-call": "off",
      "@typescript-eslint/no-unsafe-member-access": "off",
      "@typescript-eslint/no-unsafe-return": "off",
      "@typescript-eslint/prefer-promise-reject-errors": "off",
      "@typescript-eslint/require-await": "off",
      complexity: "off",
      "max-lines-per-function": "off"
    }
  }
)
