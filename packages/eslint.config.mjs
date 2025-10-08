import unusedImports from 'eslint-plugin-unused-imports'
import { configs } from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'
import { importX } from 'eslint-plugin-import-x'
import eslintConfigPrettier from "eslint-config-prettier/flat";

export default defineConfig([
  ...configs.recommended,
  importX.flatConfigs.recommended,
  importX.flatConfigs.typescript,
  eslintConfigPrettier,
  
  globalIgnores(['**/dist', '**/gen', '**/.*', '**/*.test.ts', '**/node_modules']),
  {
    plugins: {
      'unused-imports': unusedImports
    },
    rules: {
      '@typescript-eslint/no-inferrable-types': ['off'],
      '@typescript-eslint/no-empty-function': ['off'],
      '@typescript-eslint/no-unused-vars': ['off'],
      '@typescript-eslint/no-this-alias': ['off'],
      '@typescript-eslint/no-non-null-assertion': ['off'],
      '@typescript-eslint/no-explicit-any': ['off'],

      '@typescript-eslint/ban-ts-comment': [
        'error',
        {
          'ts-ignore': 'allow-with-description',
          'ts-expect-error': 'allow-with-description'
        }
      ],

      'import-x/no-extraneous-dependencies': ['off'],
      'import-x/no-named-as-default': ['off'],
      'import-x/no-unresolved': ['off'],
      'unused-imports/no-unused-imports': 'error'
    }
  }
])
