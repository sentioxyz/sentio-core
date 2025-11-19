const assetPrefix = process.env.NODE_ENV === 'development' ? '' : 'https://remix.sentio.xyz'

export enum EthChainName {
  ETHEREUM = 'Ethereum Mainnet',
  POLYGON = 'Polygon',
  MOONBEAM = 'Moonbeam',
  ASTAR = 'Astar Network'
}

export const EthChainIds = {
  [EthChainName.ETHEREUM]: '1',
  [EthChainName.POLYGON]: '137',
  [EthChainName.MOONBEAM]: '1284',
  [EthChainName.ASTAR]: '592'
}

export const EthChainLogos = {
  [EthChainName.ETHEREUM]: `${assetPrefix}/ethereum.webp`,
  [EthChainName.POLYGON]: `${assetPrefix}/polygon.webp`,
  [EthChainName.MOONBEAM]: `${assetPrefix}/moonbeam.webp`,
  [EthChainName.ASTAR]: `${assetPrefix}/astar.webp`
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
  },
  '1284': {
    unit: 'GLMR',
    decimal: 18
  },
  '592': {
    unit: 'ASTR',
    decimal: 18
  }
}
