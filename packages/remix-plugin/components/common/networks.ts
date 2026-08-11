const assetPrefix =
  process.env.NODE_ENV === 'development' ? '' : 'https://remix.sentio.xyz'

export enum EthChainName {
  ETHEREUM = 'Ethereum Mainnet',
  POLYGON = 'Polygon'
}

export const EthChainIds = {
  [EthChainName.ETHEREUM]: '1',
  [EthChainName.POLYGON]: '137'
}

export const EthChainLogos = {
  [EthChainName.ETHEREUM]: `${assetPrefix}/ethereum.webp`,
  [EthChainName.POLYGON]: `${assetPrefix}/polygon.webp`
}

export const ChainDecimals: Record<
  string,
  {
    unit: string
    decimal: number
  }
> = {
  '1': {
    unit: 'ETH',
    decimal: 18
  },
  '137': {
    unit: 'MATIC',
    decimal: 18
  }
}
