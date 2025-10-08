import { ChainInfo } from './chain-info'

export function getChainName(
  chainId: string | number | null | undefined
): string {
  if (typeof chainId === 'number') {
    chainId = chainId.toString()
  }
  if (chainId) {
    const name = ChainInfo[chainId]?.name
    if (name) {
      return name
    }
  }

  if (typeof chainId === 'string') {
    const parts = chainId.split('_')
    if (parts.length > 1) {
      return parts
        .map((part) => {
          return part[0].toUpperCase() + part.slice(1).toLowerCase()
        })
        .join(' ')
    }
  }

  return chainId || ''
}
