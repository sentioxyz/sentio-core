import { sentioTxUrl } from '~/utils/url'
import React, { ReactElement } from 'react'
import { createRoot } from 'react-dom/client'
import { TransactionCard } from '../components/transaction/TransactionCard'
import { ErrorBoundaryWrapper } from '~/content/lib/log/ErrorBoundary'
import { checkInjectElementAndLog } from '~/content/lib/log/query-element'
import {
  hexToUTF8,
  parseDataURI,
  parseTransfers
} from '~/content/lib/ethscriptions/util'
import { EthscriptionCreate } from '../components/ethscription/Creation'
import { EthscriptionTransfer } from '../components/ethscription/Transfer'

function injectSentioTxLink(chainId, hash) {
  const txHashCol = document.querySelector('#spanTxHash')?.closest('div')
  checkInjectElementAndLog(txHashCol, 'sentio tx link')

  const sentioLink = document.createElement('a')
  sentioLink.title = 'open in Sentio'
  sentioLink.style.marginLeft = '0.5rem'
  sentioLink.style.display = 'inline-block'

  const sentioIcon = document.createElement('img')
  sentioIcon.src = new URL(chrome.runtime.getURL('images/logo.png')).toString()
  sentioIcon.style.height = '1rem'
  sentioIcon.style.width = '1rem'
  sentioIcon.style.marginRight = '0.25rem'
  sentioIcon.style.verticalAlign = 'text-top'

  sentioLink.appendChild(sentioIcon)

  const sentioLinkText = document.createTextNode('Sentio')
  sentioLink.appendChild(sentioLinkText)

  sentioLink.href = sentioTxUrl(chainId, hash)
  sentioLink.rel = 'noopener nofollow'
  sentioLink.target = '_blank'
  sentioLink.className = 'flex items-center space-x-2'

  txHashCol?.append(sentioLink)
}

function injectViewFundflowButton() {
  const txHashCol = document.querySelector('#spanTxHash')?.closest('div')
  checkInjectElementAndLog(txHashCol, 'view fundflow button')
  const viewFundflowButton = document.createElement('a')
  const icon = document.createElement('i')
  icon.style.marginRight = '0.25rem'
  icon.className = 'fa fa-hashtag'
  viewFundflowButton.appendChild(icon)
  viewFundflowButton.innerHTML += 'Fundflow'
  viewFundflowButton.id = 'view-sentio-fundflow'
  viewFundflowButton.onclick = function (event) {
    event.preventDefault()
    event.stopPropagation()
    const element = document.getElementById('txn-card')
    if (element) {
      const rect = element.getBoundingClientRect()
      const absoluteTop = rect.top + window.scrollY
      const middle = absoluteTop - 60
      window.scrollTo(0, middle)
    }
  }
  viewFundflowButton.href = ''
  viewFundflowButton.className = 'mx-2 text-sm'
  viewFundflowButton.style.cursor = 'pointer'
  viewFundflowButton.title = 'view fundflow'
  txHashCol?.append(viewFundflowButton)
}

function injectViewEthscriptionButton(type: 'Mint' | 'Transfer') {
  const txHashCol = document.querySelector('#spanTxHash')?.closest('div')
  checkInjectElementAndLog(txHashCol, 'view ethscription button')
  const badgeNode = document.createElement('span')
  badgeNode.className =
    'badge bg-secondary bg-opacity-10 border border-secondary border-opacity-25 text-dark fw-medium text-start text-wrap py-1.5 px-2'
  badgeNode.innerText = `${type} Ethscriptions`
  badgeNode.style.marginLeft = '8px'
  badgeNode.style.verticalAlign = 'bottom'
  txHashCol?.append(badgeNode)
  const viewInscriptionButton = document.createElement('a')
  const icon = document.createElement('i')
  icon.style.marginRight = '0.25rem'
  icon.className = 'fa fa-layer-group'
  viewInscriptionButton.appendChild(icon)
  viewInscriptionButton.innerHTML += 'Check Ethscription Details'
  viewInscriptionButton.onclick = function () {
    const element = document.getElementById('ethscription-card')
    if (element) {
      const rect = element.getBoundingClientRect()
      const absoluteTop = rect.top + window.scrollY
      const middle = absoluteTop - 60
      window.scrollTo(0, middle)
    }
  }
  viewInscriptionButton.className = 'mx-2 text-sm'
  viewInscriptionButton.style.cursor = 'pointer'
  viewInscriptionButton.title = 'view ethscription details'
  txHashCol?.append(viewInscriptionButton)
}

// function addTab(host, id, title, content) {
//   const pane = document.createElement('div')
//   pane.className = 'tab-pane fade _sentio_'
//   pane.role = 'tabpanel'
//   pane.tabIndex = 0
//   let rendered = false
//   const renderPane = () => {
//     if (rendered) {
//       return
//     }
//     createRoot(pane.querySelector('div')!).render(content)
//     rendered = true
//   }

//   const logo = new URL(chrome.runtime.getURL('images/logo.png')).toString()
//   const tab = document.createElement('li')
//   tab.className = 'nav-item'
//   tab.role = 'presentation'
//   switch (host) {
//     case 'etherscan.io':
//     case 'goerli.etherscan.io':
//     case 'sepolia.etherscan.io':
//     case 'bscscan.com':
//     case 'lineascan.build':
//       pane.id = `${id}-tab-content`
//       pane.innerHTML = '<div class="card p-5"></div>'
//       document.querySelector('#pills-tabContent')?.append(pane)
//       document.querySelector('#ContentPlaceHolder1_myTab li:last-child')?.before(tab)
//       break
//     case 'polygonscan.com':
//     case 'zkevm.polygonscan.com':
//     case 'moonscan.io':
//       pane.id = id
//       pane.innerHTML = '<div class="card-body"></div>'
//       document.querySelector('#myTabContent')?.append(pane)
//       document.querySelector('#nav_tabs')?.append(tab)
//       break
//     default:
//       return
//   }
//   createRoot(tab).render(
//     <a
//       className="nav-link"
//       style={{ display: 'flex', alignItems: 'center' }}
//       href={`#${id}`}
//       id={`${id}-tab`}
//       data-toggle="tab"
//       data-bs-toggle="pill"
//       data-bs-target={`#${id}-tab-content`}
//       aria-controls={`${id}-tab-content`}
//       aria-selected="false"
//       tabIndex={-1}
//       role="tab"
//       onClick={renderPane}
//     >
//       <img style={{ marginRight: 4 }} src={logo} alt="sentio-logo" width="14" height="14" />
//       {title}
//     </a>
//   )
// }

function addCard(
  host: string,
  id: string,
  title: string,
  content: ReactElement
) {
  const card = document.createElement('div')
  switch (host) {
    case 'arbiscan.io':
    case 'blastscan.io':
      card.className = '_sentio_ my-4'
      break
    default:
      card.className = 'card mt-3 _sentio_ p-5'
  }
  card.appendChild(document.createElement('div'))
  let rendered = false
  const renderPane = () => {
    if (rendered) {
      return
    }
    createRoot(card.querySelector('div')!).render(content)
    rendered = true
  }
  switch (host) {
    default: {
      card.id = `${id}-card`
      const parentElement = document.getElementById(
        'ContentPlaceHolder1_maintable'
      )
      if (parentElement) {
        parentElement?.append(card)
        renderPane()
        return
      }
    }
  }
}

function checkAndInjectEthscriptionTab(
  host: string,
  chainId: string,
  hash: string
) {
  switch (host) {
    case 'etherscan.io':
    case 'cn.etherscan.com': {
      const node = document.querySelector(
        '#rawtab textarea'
      ) as HTMLTextAreaElement
      const inputData = node?.value
      if (inputData) {
        try {
          const parsedUTFString = hexToUTF8(inputData)
          const dataUri = parseDataURI(parsedUTFString)
          if (dataUri.isValid) {
            injectViewEthscriptionButton('Mint')
            injectSentioTxLink(chainId, hash)
            addCard(
              host,
              'ethscription',
              'Ethscription Creation',
              <ErrorBoundaryWrapper>
                <EthscriptionCreate data={dataUri} hash={hash} />
              </ErrorBoundaryWrapper>
            )
            return true
          }
          const transferList = parseTransfers(inputData)
          if (transferList) {
            injectViewEthscriptionButton('Transfer')
            injectSentioTxLink(chainId, hash)
            addCard(
              host,
              'ethscription',
              'Ethscription Transfer',
              <ErrorBoundaryWrapper>
                <EthscriptionTransfer transferList={transferList} />
              </ErrorBoundaryWrapper>
            )
            return true
          }
        } catch {
          //ignore
        }
      }
      break
    }
    default:
      return false
  }
}

export function transactionPage(host, chainId, hash) {
  const injected = checkAndInjectEthscriptionTab(host, chainId, hash)
  if (injected) {
    return
  }

  injectSentioTxLink(chainId, hash)
  injectViewFundflowButton()
  addCard(
    host,
    'txn',
    'Transaction Card',
    <ErrorBoundaryWrapper>
      <TransactionCard hash={hash} chainId={chainId} />
    </ErrorBoundaryWrapper>
  )
}
