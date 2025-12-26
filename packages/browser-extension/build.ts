import esbuild, { BuildOptions, Plugin } from 'esbuild'
import { Browser, genManifest } from './manifest.js'
import stylePlugin from 'esbuild-style-plugin'
import tailwind from 'tailwindcss'
import tailwindNesting from 'tailwindcss/nesting/index.js'
import prefixer from 'postcss-prefix-selector'
import { aliasPath } from 'esbuild-plugin-alias-path'
import path from 'path'
import { fileURLToPath } from 'url'
import { createRequire } from 'module'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const require = createRequire(import.meta.url)

// Plugin to resolve React from node_modules
const resolveReactPlugin: Plugin = {
  name: 'resolve-react',
  setup(build) {
    // Resolve react and react-dom to the local node_modules
    build.onResolve(
      {
        filter: /^react$|^react-dom$|^react\/jsx-runtime$|^react-dom\/client$/
      },
      (args) => {
        try {
          const resolved = require.resolve(args.path, { paths: [__dirname] })
          return {
            path: resolved
          }
        } catch (e) {
          console.error(`Failed to resolve ${args.path}:`, e)
          return null
        }
      }
    )
  }
}

const fixImportMetaPlugin: Plugin = {
  name: 'fix-import-meta',
  setup(build) {
    build.onLoad({ filter: /\.js$/ }, async (args) => {
      const fs = await import('fs/promises')
      let text = await fs.readFile(args.path, 'utf8')

      text = text.replace(
        /new URL\(([^)]+), import\.meta\.url\)/g,
        (_, p1) => `chrome.runtime.getURL(${p1})`
      )

      return { contents: text, loader: 'js' }
    })
  }
}

async function main() {
  const dirname = process.cwd()
  await genManifest(process.env.BROWSER as Browser)
  const options: BuildOptions = {
    entryPoints: ['src/background.ts', 'src/content/etherscan/main.ts'],
    bundle: true,
    packages: 'bundle',
    platform: 'browser',
    format: 'iife',
    mainFields: ['browser', 'module', 'main'],
    conditions: ['browser'],
    outdir: 'out',
    loader: {
      '.ts': 'tsx',
      '.ttf': 'dataurl'
    },
    jsx: 'automatic', // Use React 17+ automatic JSX runtime
    logLevel: 'info',
    plugins: [
      resolveReactPlugin, // Add React resolver first
      fixImportMetaPlugin,
      stylePlugin({
        postcss: {
          plugins: [
            tailwindNesting,
            tailwind,
            prefixer({
              prefix: '._sentio_',
              exclude: [/^\._sentio_.*/]
            })
          ]
        }
      }) as any,
      aliasPath({
        alias: {
          'next/router': `${dirname}/src/next/router.ts`,
          'next/link': `${dirname}/src/next/link.ts`,
          'next/font/google': `${dirname}/src/next/font/google.ts`,
          'next/dynamic': `${dirname}/src/next/dynamic.ts`,
          './logo.css': `${dirname}/src/next/logo.css`,
          'posthog-js': `${dirname}/src/next/posthog.ts`
        }
      })
    ],
    external: [],
    define: {
      MIXPANEL_TOKEN: '"8ef3bd91cdefef79e6063dcd80cb369c"',
      global: 'globalThis',
      'import.meta.env.MODE': '"production"',
      'import.meta.env': '{}',
      'import.meta': '{}'
    }
  }

  if (process.env.NODE_ENV === 'production') {
    options.minify = true
    options.define = {
      MIXPANEL_TOKEN: '"fb250c0e249067bccdf8befc84afab27"',
      global: 'globalThis'
    }
    await esbuild.build(options)
    return
  }

  const ctx = await esbuild.context(options)
  await ctx.watch()
}

main()
