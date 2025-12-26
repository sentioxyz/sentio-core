import { defineConfig, globalIgnores } from "eslint/config";
import { fixupConfigRules } from "@eslint/compat";
import tsParser from "@typescript-eslint/parser";
import path from "node:path";
import { fileURLToPath } from "node:url";
import js from "@eslint/js";
import { FlatCompat } from "@eslint/eslintrc";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const compat = new FlatCompat({
    baseDirectory: __dirname,
    recommendedConfig: js.configs.recommended,
    allConfig: js.configs.all
});

export default defineConfig([globalIgnores(["**/inject.js", "**/out", "**/dist"]), {
    extends: fixupConfigRules(compat.extends(
        "eslint:recommended",
        "plugin:react/recommended",
        "plugin:react-hooks/recommended",
        "plugin:@typescript-eslint/recommended",
        "prettier",
    )),

    languageOptions: {
        parser: tsParser,
        ecmaVersion: "latest",
        sourceType: "module",
    },

    rules: {
        "react/jsx-key": "off",
        "react/react-in-jsx-scope": "off",
        "react-hooks/exhaustive-deps": ["off"],
        "@typescript-eslint/no-unused-vars": ["off"],
        "@typescript-eslint/no-explicit-any": ["off"],
        "@typescript-eslint/no-non-null-assertion": ["off"],
        "@typescript-eslint/no-var-requires": ["off"],
        "@typescript-eslint/no-require-imports": ["off"],
        "@typescript-eslint/no-unused-expressions": ["off"],

        "@typescript-eslint/ban-ts-comment": ["error", {
            "ts-ignore": "allow-with-description",
            "ts-expect-error": "allow-with-description",
        }],
    },
}]);