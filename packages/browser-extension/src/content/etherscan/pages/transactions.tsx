import { createRoot } from 'react-dom/client'
import { addAnalytics, addBlockIndex } from '../injectors/analytic-btn'
import { ErrorBoundaryWrapper } from '~/content/lib/log/ErrorBoundary'
import { CopyButton } from '@sentio/ui-core'

function addCopyButton(col, iconClass) {
  if (col.querySelector('.js-clipboard')) {
    // Do not add copy button if there is already one.
    return
  }

  // https://etherscan.io/tx/<txn-hash>
  const hash = col.querySelector('a')?.href.split('/')[4]
  const btnCopyHash = document.createElement('div')
  btnCopyHash.className = '_sentio_'
  btnCopyHash.style.display = 'inline-block'
  btnCopyHash.style.position = 'relative'
  btnCopyHash.style.verticalAlign = 'middle'
  col.querySelector('.hash-tag')?.after(btnCopyHash)
  createRoot(btnCopyHash).render(
    <ErrorBoundaryWrapper fallback={<span></span>}>
      <div className="absolute top-[-10px]">
        <CopyButton text={hash} size={14} className={iconClass} />
      </div>
    </ErrorBoundaryWrapper>
  )
}

function addCopyButtons(host) {
  const iconClass = 'text-gray-500'
  const rows = document.querySelectorAll('tbody tr')
  for (let i = 0; i < rows.length; i++) {
    const row = rows[i] as HTMLTableRowElement
    addCopyButton(row.cells[1], iconClass) // Txn Hash

    if (host === 'polygonscan.com') {
      addCopyButton(row.cells[6], iconClass) // From
      addCopyButton(row.cells[8], iconClass) // To
    }
  }
}

function checkAndAddCopyButtons(host) {
  setTimeout(() => {
    const row = document.querySelector('tbody tr')
    const colHash = (row as HTMLTableRowElement).cells[3] // Block
    if (!colHash.querySelector('svg')) {
      // Do not add copy button if there is already one.
      addCopyButtons(host)
    }
  }, 1000)
}

export function transactionsPage(host, chainId) {
  checkAndAddCopyButtons(host)

  let transactionsPanel
  let fontAwesome5 = false
  switch (host) {
    case 'arbiscan.io':
    case 'blastscan.io':
      fontAwesome5 = true
      transactionsPanel = document.querySelector('#ContentPlaceHolder1_mainrow')
      break
    default:
      transactionsPanel = document.querySelector(
        '#ContentPlaceHolder1_divTransactions'
      )
      fontAwesome5 = false
  }
  addAnalytics(chainId, transactionsPanel, fontAwesome5)
  addBlockIndex(chainId, transactionsPanel)
}
