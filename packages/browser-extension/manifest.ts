import fs from 'fs/promises'

export type Browser = 'chrome' | 'firefox'

const backgroundScript = 'out/background.js'

export async function genManifest(browser: Browser = 'chrome') {
  const manifest: any = {
    name: 'Sentio',
    version: '0.50.1',
    description:
      'Modern monitoring, alerting, log management and debugging for decentralized applications.',
    manifest_version: 3,
    content_scripts: [
      {
        matches: [
          'https://etherscan.io/*',
          'https://cn.etherscan.com/*',
          'https://polygonscan.com/*',
          'https://holesky.etherscan.io/*',
          'https://sepolia.etherscan.io/*',
          'https://bscscan.com/*',
          'https://lineascan.build/*',
          'https://moonscan.io/*',
          'https://scrollscan.com/*',
          'https://arbiscan.io/*',
          'https://blastscan.io/*',
          'https://basescan.org/*',
          'https://hoodi.etherscan.io/*',
          'https://optimistic.etherscan.io/*',
          'https://sonicscan.org/*',
          'https://taikoscan.io/*',
          'https://berascan.com/*',
          'https://hyperevmscan.io/*'
        ],
        js: ['out/content/etherscan/main.js'],
        css: ['out/content/etherscan/main.css'],
        run_at: 'document_end'
      },
      {
        matches: ['https://app.sentio.xyz/*'],
        js: ['out/content/etherscan/main.js'],
        run_at: 'document_end'
      }
    ],
    host_permissions: [
      // 'https://etherscan.io/*',
      // 'https://polygonscan.com/*',
      // 'https://moonbeam.subscan.io/*',
      // 'https://astar.subscan.io/*',
      // 'https://blockscout.com/*',
      // 'https://goerli.etherscan.io/*',
      // 'https://sepolia.etherscan.io/*',
      // 'https://bscscan.com/*',
      // 'https://zkevm.polygonscan.com/*',
      // 'https://lineascan.build/*',
      // 'https://moonscan.io/*'
    ],
    web_accessible_resources: [
      {
        resources: ['images/*'],
        matches: ['<all_urls>']
      }
    ],
    icons: {
      128: '/images/logo.png'
    },
    permissions: ['storage'],
    externally_connectable: {
      matches: ['*://*.sentio.xyz/*']
    }
  }

  switch (browser) {
    case 'chrome':
      manifest.background = {
        service_worker: backgroundScript
      }
      break
    case 'firefox':
      manifest.background = {
        scripts: [backgroundScript]
      }
      break
  }

  fs.writeFile('manifest.json', JSON.stringify(manifest, null, 2))
}
