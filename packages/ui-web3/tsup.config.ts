import { defineConfig } from 'tsup'

export default defineConfig({
  entry: ['src/index.ts'],
  format: ['esm', 'cjs'],
  dts: false, // Disable DTS generation for now due to external dependencies
  sourcemap: true,
  external: [
    'react',
    'react-dom',
    '@sentio/ui-core',
    '@sentio/scip',
    '@monaco-editor/react',
    'monaco-editor',
    'gen/service/solidity/protos/service.pb',
    'lib/data/use-chain-config',
    'lib/data/with-json-api'
  ]
})
