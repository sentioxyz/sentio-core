import { createRoot } from 'react-dom/client'
import { loader } from '@monaco-editor/react'
import * as monaco from 'monaco-editor'
import { sentioContractUrl } from '~/utils/url'
import { addAnalytics, addBlockIndex } from '../injectors/analytic-btn'
import { ScipSolidity } from '@sentio/scip'
import { CodeEditor } from '../components/contract/CodeEditor'
import { ErrorBoundaryWrapper } from '../../lib/log/ErrorBoundary'
import { checkInjectElementAndLog } from '~/content/lib/log/query-element'
import { CodeEditorNavigator } from '../components/contract/CodeEditorNavigator'
import {
  SourceStore,
  parseUri,
  setSolidityLanguage,
  setSolidityProviders
} from '@sentio/ui-web3'

// Inject Sentio to the left of the "Open In" button.
function injectSentioLink(chainId, address, host) {
  const openInDiv = document.querySelector('.mt-1.mt-md-0')
  checkInjectElementAndLog(openInDiv, 'inject sentio link')

  const sentioButton = document.createElement('div')
  sentioButton.style.display = 'inline-block'
  sentioButton.style.verticalAlign = 'bottom'
  sentioButton.style.marginLeft = '0.5rem'
  const logo = new URL(chrome.runtime.getURL('images/logo.png')).toString()
  switch (host) {
    case 'etherscan.io':
    case 'cn.etherscan.com':
    case 'holesky.etherscan.io':
    case 'sepolia.etherscan.io':
    case 'bscscan.com':
      sentioButton.innerHTML = /* HTML */ `<a
        class="btn btn-sm btn-secondary"
        style="display: flex; align-items: center"
        href="${sentioContractUrl(chainId, address)}"
        rel="noopener nofollow"
        target="_blank"
      >
        <img
          class="me-1"
          src="${logo}"
          alt="sentio-logo"
          width="14"
          height="14"
        />
        Open in Sentio
      </a>`
      break
    case 'polygonscan.com':
    case 'zkevm.polygonscan.com':
    case 'lineascan.build':
    case 'moonscan.io':
      sentioButton.innerHTML = /* HTML */ `<a
        class="btn btn-sm btn-secondary btn-xss ml-1"
        style="display: flex; align-items: center"
        href="${sentioContractUrl(chainId, address)}"
        rel="noopener nofollow"
        target="_blank"
      >
        <img
          class="me-1 mr-1"
          src="${logo}"
          alt="sentio-logo"
          width="16"
          height="16"
        />
        Open in Sentio
      </a>`
      break
  }
  openInDiv?.append(sentioButton)
}

export function getFileUri(id: string, path: string) {
  return monaco.Uri.parse(`file:///${id}/${path}`)
}

async function renderCodeEditors(chainId, address) {
  setSolidityLanguage(monaco)
  loader.config({ monaco })

  const [sourceData, contractIndex, monacoInstance] = await Promise.all([
    chrome.runtime.sendMessage({
      api: 'FetchAndCompile',
      address,
      chainId
    }),
    chrome.runtime.sendMessage({
      api: 'GetContractIndex',
      address,
      chainId
    }),
    loader.init()
  ])

  const { sources } = sourceData.result[0]
  const scip = new ScipSolidity(sources, contractIndex.index)
  const sourceStore = new SourceStore(
    sourceData,
    chainId,
    Promise.resolve(scip)
  )
  setSolidityProviders(monacoInstance, sourceStore, parseUri)
  const history: any[] = []

  // render navigators
  const parentElement = document.querySelector('#ethPrice')
  const node = document.createElement('div')
  let root: any
  if (parentElement) {
    parentElement.after(node)
  }
  function addHistory(data: any) {
    history.push(data)
    if (!root) {
      root = createRoot(node)
    }
    root.render(
      <ErrorBoundaryWrapper>
        <CodeEditorNavigator history={[...history]} />
      </ErrorBoundaryWrapper>
    )
  }

  sources.forEach((source) => {
    monaco.editor.createModel(
      source.source,
      'sentio-solidity',
      getFileUri(address, source.sourcePath)
    )
  })

  const editors = document.querySelectorAll('#dividcode .ace_editor')
  checkInjectElementAndLog(editors, 'replace ace editor')
  const pathToEditorTitle = {}
  const scrollIntoView = (path: string) => {
    // pathToEditorTitle[path]?.scrollIntoView()
    const rect = pathToEditorTitle[path]?.getBoundingClientRect()
    if (!rect) {
      return
    }
    window.scrollBy(0, rect.top - 100)
  }

  for (const editor of editors) {
    let model: null | monaco.editor.ITextModel = null
    try {
      if (editors.length > 1) {
        const name = editor.previousSibling?.firstChild?.textContent
          ?.split(':')[1]
          ?.trim()
        if (!name && editors.length > 1) {
          continue
        }
        const source = sources.find((source) => {
          const fileName = source.sourcePath.split('/').pop()
          return fileName === name
        })
        model = monaco.editor.getModel(getFileUri(address, source.sourcePath))
      } else if (editors.length === 1) {
        model = monaco.editor.getModel(
          getFileUri(address, sources[0].sourcePath)
        )
      }
    } catch {
      // ignore
    }

    if (model) {
      const sentioEditor = document.createElement('div')
      sentioEditor.className = '_sentio_'
      editor.append(sentioEditor)
      createRoot(sentioEditor).render(
        <ErrorBoundaryWrapper>
          <CodeEditor
            path={model.uri.path}
            model={model}
            scrollIntoView={scrollIntoView}
            chainId={chainId}
            addHistory={addHistory}
          />
        </ErrorBoundaryWrapper>
      )
      pathToEditorTitle[model.uri.path] = editor.previousSibling
    }
  }
}

function render(host, chainId, address) {
  renderCodeEditors(chainId, address)
  injectSentioLink(chainId, address, host)
  return
}

export function tokenPage(host, chainId, address) {
  /**
   * render contracts page
   */
  const codePanel = document.querySelector('#contracts')
  if (codePanel) {
    if (document.location.hash === '#code') {
      render(host, chainId, address)
    } else {
      const observer = new MutationObserver(() => {
        if (document.location.hash === '#code') {
          render(host, chainId, address)
          observer.disconnect()
        }
      })
      observer.observe(codePanel, {
        attributes: true
      })
    }
  }
  let fontAwesome5 = false
  switch (host) {
    case 'arbiscan.io':
    case 'blastscan.io':
      fontAwesome5 = true
      break
    default:
      fontAwesome5 = false
  }
  addAnalytics(chainId, document.querySelector('#transactions'), fontAwesome5)
  addBlockIndex(chainId, document.querySelector('#transactions'))
}
