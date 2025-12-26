import { createRoot } from 'react-dom/client'
import { TransactionSlideOver } from '../components/transaction/TransactionSlideOver'
import { ErrorBoundaryWrapper } from '../../lib/log/ErrorBoundary'
import { checkInjectElementAndLog } from '~/content/lib/log/query-element'
import { getTransactions } from '~/content/lib/debug/use-transaction-info'
import { formatCurrency } from '@sentio/ui-web3'

function pLimit(concurrency: number) {
  let active = 0
  const queue: (() => void)[] = []
  return (fn) =>
    new Promise((resolve, reject) => {
      const run = () => {
        active++
        Promise.resolve(fn())
          .then(resolve, reject)
          .finally(() => {
            active--
            if (queue.length > 0) {
              const runInstance = queue.shift()
              runInstance && runInstance()
            }
          })
      }
      if (active < concurrency) {
        run()
      } else {
        queue.push(run)
      }
    })
}

export function addAnalytics(
  chainId: string,
  transactionsPanel: Element | null,
  fontAwesome5?: boolean
) {
  if (transactionsPanel) {
    const tableRows = transactionsPanel.querySelectorAll('table tbody tr')
    checkInjectElementAndLog(tableRows, 'analytics button')
    const hashList: string[] = []
    const buttonList: Map<string, any> = new Map()
    tableRows.forEach((_row) => {
      const row = _row as HTMLTableRowElement
      const previewCell = row.cells[0]
      const hashElement = row.querySelector(
        '.myFnExpandBox_searchVal'
      ) as HTMLAnchorElement
      const hash = hashElement?.href?.split('/tx/')[1]
      previewCell.style.display = 'flex'
      previewCell.style.gap = '2px'
      previewCell.style.alignItems = 'center'
      let button
      if (fontAwesome5) {
        button = document.createElement('a')
        button.innerHTML =
          '<i class="far fa-search-location btn-icon__inner"></i>'
        button.role = 'button'
        button.type = 'button'
        button.className =
          'js-txnAdditional-1 btn btn-xs btn-icon btn-soft-secondary'
      } else {
        button = document.createElement('button')
        button.className = 'btn btn-sm btn-white content-center mx-auto'
        const icon = document.createElement('i')
        icon.className = 'fa-solid fa-magnifying-glass-chart'
        button.style.width = '1.75rem'
        button.style.height = '1.75rem'
        button.appendChild(icon)
      }
      previewCell.appendChild(button)
      if (hash && chainId === '1') {
        hashList.push(hash)
        buttonList.set(hash, button)
      }
      button.onclick = () => {
        const global = window as any
        if (global.openSlideOver && hash && chainId) {
          global.openSlideOver(hash, chainId)
        }
      }
      button.title = 'View Analytics by Sentio'
    })
    const sidePanel = document.createElement('div')
    transactionsPanel.appendChild(sidePanel)
    createRoot(sidePanel).render(
      <ErrorBoundaryWrapper>
        <TransactionSlideOver />
      </ErrorBoundaryWrapper>
    )
    chrome.runtime
      .sendMessage({
        api: 'GetMevBatch',
        chainId: chainId,
        hashList: hashList
      })
      .then((res) => {
        res?.results?.forEach((item) => {
          if (item.type === 'NONE') {
            return
          }
          if (item.type) {
            const mevType = item.sandwich ? 'Sandwich' : 'Arbitrage'
            if (mevType === 'Arbitrage') {
              return
            }
            if (
              mevType === 'Sandwich' &&
              (item.sandwich.frontTxHash === item.txHash ||
                item.sandwich.backTxHash === item.txHash)
            ) {
              return
            }
            const btn = buttonList.get(item.txHash)
            if (btn) {
              const valueLoss = formatCurrency(
                item.sandwich?.revenues?.totalUsd ||
                  item.arbitrage?.revenues?.totalUsd
              )
              btn.style.color = 'rgb(234 88 12)'
              btn.title = `View Analytics by Sentio (${mevType} detected, estimated value loss: ${valueLoss})`
            }
          }
        })
      })
      .catch((e) => {
        // ignore
      })
  }
}

const indexElementStyle = {
  marginLeft: '4px',
  fontSize: '0.75rem',
  color: 'var(--bs-secondary-color)'
}

export function addBlockIndex(
  chainId: string,
  transactionsPanel: Element | null
) {
  if (transactionsPanel) {
    const tableRows = transactionsPanel.querySelectorAll('table tbody tr')
    checkInjectElementAndLog(tableRows, 'block index')
    const groupedRows: {
      element: HTMLTableCellElement
      blockNumber: string
      hash: string
    }[] = []
    tableRows.forEach((_row) => {
      const row = _row as HTMLTableRowElement
      const blockCell = row.cells[3]
      const hashElement = row.querySelector(
        '.myFnExpandBox_searchVal'
      ) as HTMLAnchorElement
      const hash = hashElement?.href?.split('/tx/')[1]
      if (blockCell?.innerText !== '(pending)' && hash) {
        groupedRows.push({
          element: blockCell,
          blockNumber: blockCell.innerText,
          hash
        })
      }
    })
    const limit = pLimit(1)
    let list: string[] = []
    const elementMap = new Map()
    groupedRows.forEach((item, index) => {
      // batch requests into one
      list.push(item.hash)
      elementMap.set(item.hash, item.element)
      if (list.length === 20 || index === groupedRows.length - 1) {
        limit(async () => {
          try {
            const txHashList = [...list]
            list = []
            const txs = await getTransactions(txHashList, chainId)
            Object.entries(txs).forEach(([hash, tx]) => {
              const { transactionIndex } = tx as any
              const index = Number.parseInt(transactionIndex, 16)
              const element = elementMap.get(hash)
              const containerEl = document.createElement('span')
              Object.assign(containerEl.style, indexElementStyle)
              containerEl.innerText = `(index: ${index})`
              element.appendChild(containerEl)
            })
          } catch (e: any) {
            // ignore
          }
        })
      }
    })
  }
}
