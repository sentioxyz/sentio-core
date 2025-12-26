import { createRoot } from 'react-dom/client'
import { CopyButton } from '@sentio/ui-core'

export function blockPage(host, chainId, address) {
  /**
   * render block page
   */
  let blockHeightRowNode: HTMLDivElement | null = null
  document.querySelectorAll('div.row').forEach((node) => {
    const element = node as HTMLDivElement
    if (element.textContent?.includes('Block Height')) {
      blockHeightRowNode = element
    }
  })
  if (!blockHeightRowNode) {
    return
  }
  if (
    (blockHeightRowNode as HTMLDivElement).children[1]?.children[0]?.children[0]
  ) {
    const rootNode = document.createElement('div')
    rootNode.style.display = 'inline-block'
    rootNode.style.marginLeft = '4px'
    rootNode.style.verticalAlign = 'text-bottom'
    rootNode.className = '_sentio_'
    ;(
      blockHeightRowNode as HTMLDivElement
    ).children[1].children[0]?.children[0].appendChild(rootNode)
    createRoot(rootNode).render(<CopyButton text={address} size={14} />)
  }
}
