import { EthChainId } from '@sentio/chain'
import { tokenPage } from './pages/token'
import { transactionPage } from './pages/transaction'
import { transactionsPage } from './pages/transactions'
import { blockPage } from './pages/block'
import '@sentio/ui-core/dist/style.css'
import './main.css'
import { isSentioPage } from './sentio/util'
import { ProjectPage } from './sentio/project'

const injectedStyle = `
#headlessui-portal-root {
  z-index: 2000;
}`

async function main() {
  if (isSentioPage()) {
    return ProjectPage()
  }

  // inject style into the page
  const style = document.createElement('style')
  style.innerHTML = injectedStyle
  document.head.appendChild(style)

  // init set theme
  const theme = document.documentElement.getAttribute('data-bs-theme')
  if (theme === 'dark' || theme === 'dim') {
    document.body.classList.add('dark')
  }
  // theme observer, watch html node property data-bs-theme changes
  const observer = new MutationObserver(() => {
    const theme = document.documentElement.getAttribute('data-bs-theme')
    if (theme === 'dark' || theme === 'dim') {
      document.body.classList.add('dark')
    } else {
      document.body.classList.remove('dark')
    }
  })
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['data-bs-theme']
  })

  const { host, pathname, hash } = document.location
  const [, page, address] = pathname.split('/')
  const chainId =
    {
      'etherscan.io': EthChainId.ETHEREUM,
      'cn.etherscan.com': EthChainId.ETHEREUM,
      'polygonscan.com': EthChainId.POLYGON,
      'holesky.etherscan.io': EthChainId.HOLESKY,
      'sepolia.etherscan.io': EthChainId.SEPOLIA,
      'bscscan.com': EthChainId.BSC,
      'lineascan.build': EthChainId.LINEA,
      'moonscan.io': EthChainId.MOONBEAM,
      'scrollscan.com': EthChainId.SCROLL,
      'arbiscan.io': EthChainId.ARBITRUM,
      'blastscan.io': EthChainId.BLAST,
      'basescan.org': EthChainId.BASE,
      'hoodi.etherscan.io': EthChainId.HOODI,
      'optimistic.etherscan.io': EthChainId.OPTIMISM,
      'sonicscan.org': EthChainId.SONIC_MAINNET,
      'taikoscan.io': EthChainId.TAIKO,
      'berascan.com': EthChainId.BERACHAIN,
      'hyperevmscan.io': EthChainId.HYPER_EVM
    }[host] || '1'
  try {
    switch (page) {
      case 'address':
      case 'token':
        return tokenPage(host, chainId, address)
      case 'tx':
        return transactionPage(host, chainId, address)
      case 'txs':
        return transactionsPage(host, chainId)
      case 'block':
        return blockPage(host, chainId, address)
    }
  } catch (e: any) {
    // ignore
  }
}

main()
