import path from 'node:path'

// This Vite config only affects Ladle (ui-dashboard itself builds with tsup).
//
// @sentio/chain ships CommonJS and is a symlinked workspace dependency. Two
// problems in Ladle's Vite dev server:
//   1. Vite excludes linked workspace deps from pre-bundling, so it serves the
//      raw CJS file and the browser's native ESM can't read its named exports
//      (`import { getChainName } from '@sentio/chain'` throws at runtime).
//   2. Ladle runs Vite with its own internal root, so a bare-specifier
//      optimizeDeps.include can't be resolved ("Failed to resolve dependency").
//
// Alias the package to its built entry by absolute path (root-independent), and
// force it into optimizeDeps so esbuild pre-bundles it to ESM with named
// exports. `pnpm ladle` runs from this package dir, so cwd is ui-dashboard.
const chainEntry = path.resolve(process.cwd(), '../chain/dist/index.js')

export default {
  resolve: {
    alias: {
      '@sentio/chain': chainEntry
    }
  },
  optimizeDeps: {
    include: ['@sentio/chain']
  }
}
