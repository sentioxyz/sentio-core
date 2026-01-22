import {
  AptosChainId,
  BTCChainId,
  ChainId,
  CosmosChainId,
  EthChainId,
  FuelChainId,
  SolanaChainId,
  StarknetChainId,
  SuiChainId
} from './chain-id'

export enum ExplorerApiType {
  ETHERSCAN = 'etherscan',
  ETHERSCAN_V2 = 'etherscan_v2',
  BLOCKSCOUT = 'blockscout',
  L2_SCAN = 'l2scan',
  OK_LINK = 'oklink',
  UNKNOWN = 'unknown'
}

export type ChainInfo = {
  name: string
  slug: string
  additionalSlugs?: string[]
  chainId: ChainId
  nativeChainId?: number
  mainnetChainId?: ChainId
  explorerUrl: string
  lightIcon?: string // icon used in light mode, default icon
  darkIcon?: string // icon used in dark mode
}

export enum EthVariation {
  DEFAULT = 0,
  ARBITRUM = 1,
  OPTIMISM = 2,
  ZKSYNC = 3,
  POLYGON_ZKEVM = 4,
  SUBSTRATE
}

export type EthChainInfo = ChainInfo & {
  mainnetChainId?: EthChainId // if it is a testnet, this is the mainnet chain id
  chainId: EthChainId
  variation: EthVariation

  tokenAddress: string // native token address
  tokenSymbol: string // native token symbol
  tokenDecimals: number // native token decimals

  priceTokenAddress: string // token address for price
  wrappedTokenAddress: string // wrapped token address with contract, normally Wxxx (Wrapped xxx)

  explorerApiType?: ExplorerApiType
  explorerApi?: string
  blockscoutUrl?: string
}

/**
 * EVM chains
 */
export const EthChainInfo: Record<EthChainId | string, EthChainInfo> = {
  [EthChainId.ETHEREUM]: {
    name: 'Ethereum',
    slug: 'mainnet',
    additionalSlugs: ['ethereum'],
    chainId: EthChainId.ETHEREUM,
    nativeChainId: 1,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerUrl: 'https://etherscan.io',
    explorerApi: 'https://api.etherscan.io/v2',
    blockscoutUrl: 'https://eth.blockscout.com',
    lightIcon: 'https://assets.sentio.xyz/chains/eth.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/eth-dark.svg'
  },
  [EthChainId.SEPOLIA]: {
    name: 'Sepolia',
    slug: 'sepolia',
    chainId: EthChainId.SEPOLIA,
    nativeChainId: 11155111,
    mainnetChainId: EthChainId.ETHEREUM,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x1ef5f52bdbe11af2377c58ecc914a8c72ea807cf',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerUrl: 'https://sepolia.etherscan.io',
    explorerApi: 'https://api.etherscan.io/v2',
    blockscoutUrl: 'https://eth-sepolia.blockscout.com',
    lightIcon: 'https://assets.sentio.xyz/chains/eth.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/eth-dark.svg'
  },
  [EthChainId.HOLESKY]: {
    name: 'Holesky',
    slug: 'holesky',
    chainId: EthChainId.HOLESKY,
    nativeChainId: 17000,
    mainnetChainId: EthChainId.ETHEREUM,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x94373a4919B3240D86eA41593D5eBa789FEF3848',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerUrl: 'https://holesky.etherscan.io',
    explorerApi: 'https://api.etherscan.io/v2',
    blockscoutUrl: 'https://eth-holesky.blockscout.com',
    lightIcon: 'https://assets.sentio.xyz/chains/eth.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/eth-dark.svg'
  },
  [EthChainId.HOODI]: {
    name: 'Hoodi',
    slug: 'hoodi',
    chainId: EthChainId.HOODI,
    nativeChainId: 560048,
    mainnetChainId: EthChainId.ETHEREUM,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerUrl: 'https://hoodi.etherscan.io',
    explorerApi: 'https://api.etherscan.io/v2',
    blockscoutUrl: 'https://light-hoodi.beaconcha.in',
    lightIcon: 'https://assets.sentio.xyz/chains/eth.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/eth-dark.svg'
  },
  [EthChainId.BSC]: {
    name: 'Binance Smart Chain',
    slug: 'bsc',
    chainId: EthChainId.BSC,
    nativeChainId: 56,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c',
    tokenSymbol: 'BNB',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerUrl: 'https://bscscan.com',
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/bnb-chain.svg'
  },
  [EthChainId.BSC_TESTNET]: {
    name: 'Binance Smart Chain Testnet',
    slug: 'bsc-testnet',
    chainId: EthChainId.BSC_TESTNET,
    nativeChainId: 97,
    mainnetChainId: EthChainId.BSC,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0xae13d989dac2f0debff460ac112a837c89baa7cd',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xae13d989dac2f0debff460ac112a837c89baa7cd',
    tokenSymbol: 'tBNB',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerUrl: 'https://testnet.bscscan.com',
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/bnb-chain.svg'
  },
  [EthChainId.OP_BNB_MAINNET]: {
    name: 'opBNB Mainnet',
    slug: 'opbnb',
    chainId: EthChainId.OP_BNB_MAINNET,
    nativeChainId: 204,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenSymbol: 'BNB',
    tokenDecimals: 18,
    explorerUrl: 'https://opbnb.bscscan.com',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/bnb-chain.svg'
  },
  [EthChainId.POLYGON]: {
    name: 'Polygon',
    slug: 'matic',
    additionalSlugs: ['polygon'],
    chainId: EthChainId.POLYGON,
    nativeChainId: 137,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0d500b1d8e8ef31e21c99d1db9a6444d3adf1270', // WMATIC
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x0d500b1d8e8ef31e21c99d1db9a6444d3adf1270', // WMATIC
    tokenSymbol: 'MATIC',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerUrl: 'https://polygonscan.com',
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/polygon.svg'
    // blockscoutBlockPrefix: 'https://polygon.blockscout.com/block/',
  },
  [EthChainId.ARBITRUM]: {
    name: 'Arbitrum',
    slug: 'arbitrum-one',
    additionalSlugs: ['arbitrum'],
    chainId: EthChainId.ARBITRUM,
    nativeChainId: 42161,
    variation: EthVariation.ARBITRUM,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x82aF49447D8a07e3bd95BD0d56f35241523fBab1',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerUrl: 'https://arbiscan.io',
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/arbitrum.svg'
  },
  [EthChainId.AVALANCHE]: {
    name: 'Avalanche',
    slug: 'avalanche',
    chainId: EthChainId.AVALANCHE,
    nativeChainId: 43114,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xB31f66AA3C1e785363F0875A1B74E27b85FD66c7',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.ETHERSCAN,
    explorerUrl: 'https://snowtrace.io',
    explorerApi:
      'https://api.routescan.io/v2/network/mainnet/evm/43114/etherscan',
    lightIcon: 'https://assets.sentio.xyz/chains/avalanche.svg'
  },
  [EthChainId.POLYGON_ZKEVM]: {
    name: 'Polygon zkEVM',
    chainId: EthChainId.POLYGON_ZKEVM,
    nativeChainId: 1101,
    slug: 'polygon-zkevm',
    variation: EthVariation.POLYGON_ZKEVM,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4F9A0e7FD2Bf6067db6994CF12E4495Df938E6e9',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerUrl: 'https://polygon.blockscout.com',
    explorerApi: 'https://polygon.blockscout.com',
    lightIcon: 'https://assets.sentio.xyz/chains/polygon.svg'
  },
  [EthChainId.MOONBEAM]: {
    name: 'Moonbeam',
    slug: 'moonbeam',
    chainId: EthChainId.MOONBEAM,
    nativeChainId: 1284,
    variation: EthVariation.SUBSTRATE,
    priceTokenAddress: '0xacc15dc74880c9944775448304b263d191c6077f', // WGLMR
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xacc15dc74880c9944775448304b263d191c6077f',
    tokenSymbol: 'GLMR',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerUrl: 'https://moonscan.io',
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/moonbeam.svg'
  },
  [EthChainId.ASTAR]: {
    name: 'Astar',
    slug: 'astar',
    chainId: EthChainId.ASTAR,
    nativeChainId: 592,
    variation: EthVariation.SUBSTRATE,
    priceTokenAddress: '0xaeaaf0e2c81af264101b9129c00f4440ccf0f720', // WASTR
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xaeaaf0e2c81af264101b9129c00f4440ccf0f720',
    tokenSymbol: 'ASTR',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerUrl: 'https://astar.blockscout.com',
    explorerApi: 'https://astar.blockscout.com',
    lightIcon: 'https://assets.sentio.xyz/chains/astar.svg'
  },
  [EthChainId.LINEA]: {
    name: 'Linea',
    slug: 'linea',
    chainId: EthChainId.LINEA,
    nativeChainId: 59144,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0xe5d7c2a44ffddf6b295a15c148167daaaf5cf34f', // WETH
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xe5D7C2a44FfDDf6b295A15c148167daaAf5Cf34f',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://lineascan.build',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/linea.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/linea-dark.svg'
  },
  [EthChainId.SCROLL]: {
    name: 'Scroll',
    slug: 'scroll',
    chainId: EthChainId.SCROLL,
    nativeChainId: 534352,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x5300000000000000000000000000000000000004', // TODO questionable
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://scrollscan.com',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/scroll.svg'
  },
  [EthChainId.TAIKO]: {
    name: 'Taiko Mainnet',
    slug: 'taiko',
    chainId: EthChainId.TAIKO,
    nativeChainId: 167000,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xA51894664A773981C6C112C43ce576f315d5b1B6',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://taikoscan.io',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/taiko.svg'
  },
  // [EthChainId.TAIKO_TESTNET]: {
  //   name: 'Taiko Testnet',
  //   slug: 'taiko-hekla-testnet',
  //   chainId: EthChainId.TAIKO_TESTNET,
  //   mainnetChainId: EthChainId.TAIKO,
  //   variation: EthVariation.DEFAULT,
  //   priceTokenAddress: '0x0000000000000000000000000000000000000000',
  //   tokenAddress: '0x0000000000000000000000000000000000000000',
  //   wrappedTokenAddress: '0xae2C46ddb314B9Ba743C6dEE4878F151881333D9',
  //   tokenSymbol: 'ETH',
  //   tokenDecimals: 18,
  //   explorerUrl: 'https://hekla.taikoscan.io',
  //   explorerApiType: ExplorerApiType.ETHERSCAN,
  //   explorerApi: 'https://api.etherscan.io/v2',
  //   lightIcon: 'https://assets.sentio.xyz/chains/taiko.svg'
  // },
  [EthChainId.XLAYER_TESTNET]: {
    name: 'X Layer Testnet',
    slug: 'xlayer-sepolia',
    chainId: EthChainId.XLAYER_TESTNET,
    nativeChainId: 195,
    mainnetChainId: EthChainId.XLAYER_MAINNET,
    variation: EthVariation.POLYGON_ZKEVM,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xa749ad81913cdc19881ebeb64631df72be708335',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://www.oklink.com/xlayer-test',
    explorerApiType: ExplorerApiType.OK_LINK,
    explorerApi: 'https://www.oklink.com/api/v5/explorer',
    lightIcon: 'https://assets.sentio.xyz/chains/x1-logo.png'
  },
  [EthChainId.CORE_MAINNET]: {
    name: 'Core',
    slug: 'core-mainnet',
    chainId: EthChainId.CORE_MAINNET,
    nativeChainId: 1116,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x40375c92d9faf44d2f9db9bd9ba41a3317a2404f',
    tokenSymbol: 'CORE',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.UNKNOWN,
    explorerUrl: 'https://scan.coredao.org',
    // explorerApi: 'https://openapi.coredao.org',
    lightIcon: 'https://assets.sentio.xyz/chains/core.svg'
  },
  [EthChainId.XLAYER_MAINNET]: {
    name: 'X Layer Mainnet',
    slug: 'xlayer-mainnet',
    chainId: EthChainId.XLAYER_MAINNET,
    nativeChainId: 196,
    variation: EthVariation.POLYGON_ZKEVM,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x5a77f1443d16ee5761d310e38b62f77f726bc71c',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://www.oklink.com/xlayer',
    explorerApiType: ExplorerApiType.OK_LINK,
    explorerApi: 'https://www.oklink.com/api/v5/explorer',
    lightIcon: 'https://assets.sentio.xyz/chains/x1-logo.png'
  },
  [EthChainId.BLAST]: {
    name: 'Blast Mainnet',
    slug: 'blast-mainnet',
    chainId: EthChainId.BLAST,
    nativeChainId: 81457,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x4300000000000000000000000000000000000004',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4300000000000000000000000000000000000004',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://blastscan.io',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/blast-logo.png'
  },
  [EthChainId.BLAST_SEPOLIA]: {
    name: 'Blast Testnet',
    slug: 'blast-testnet',
    chainId: EthChainId.BLAST_SEPOLIA,
    nativeChainId: 168587773,
    mainnetChainId: EthChainId.BLAST,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x4200000000000000000000000000000000000023',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000023',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://sepolia.blastscan.io',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/blast-logo.png'
  },
  [EthChainId.BASE]: {
    name: 'Base',
    slug: 'base',
    chainId: EthChainId.BASE,
    nativeChainId: 8453,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://basescan.org',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/base.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/base_blue.svg'
  },
  [EthChainId.BASE_SEPOLIA]: {
    name: 'Base Sepolia',
    slug: 'base-sepolia',
    chainId: EthChainId.BASE_SEPOLIA,
    nativeChainId: 84532,
    mainnetChainId: EthChainId.BASE,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://sepolia.basescan.org',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/base.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/base_blue.svg'
  },
  [EthChainId.ZKSYNC_ERA]: {
    name: 'zkSync Era',
    slug: 'zksync-era',
    chainId: EthChainId.ZKSYNC_ERA,
    nativeChainId: 324,
    variation: EthVariation.ZKSYNC,
    priceTokenAddress: '0x000000000000000000000000000000000000800A',
    tokenAddress: '0x000000000000000000000000000000000000800A',
    wrappedTokenAddress: '0x000000000000000000000000000000000000800A',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.zksync.io',
    explorerApiType: ExplorerApiType.ETHERSCAN,
    explorerApi: 'https://block-explorer-api.mainnet.zksync.io',
    lightIcon: 'https://assets.sentio.xyz/chains/zksync.svg'
  },
  [EthChainId.KATANA_MAINNET]: {
    name: 'Katana Mainnet',
    slug: 'katana',
    chainId: EthChainId.KATANA_MAINNET,
    nativeChainId: 747474,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://katanascan.com',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/katana.svg'
  },
  [EthChainId.ZIRCUIT_GARFIELD_TESTNET]: {
    name: 'Zircuit Garfield Testnet',
    slug: 'zircuit-garfield-testnet',
    chainId: EthChainId.ZIRCUIT_GARFIELD_TESTNET,
    nativeChainId: 48898,
    mainnetChainId: EthChainId.ZIRCUIT_MAINNET,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.garfield-testnet.zircuit.com',
    lightIcon: 'https://assets.sentio.xyz/chains/zircuit-inverted-icon.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/zircuit-green-icon.svg'
  },
  [EthChainId.ZIRCUIT_MAINNET]: {
    name: 'Zircuit Mainnet',
    slug: 'zircuit',
    chainId: EthChainId.ZIRCUIT_MAINNET,
    nativeChainId: 48900,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.zircuit.com',
    lightIcon: 'https://assets.sentio.xyz/chains/zircuit-inverted-icon.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/zircuit-green-icon.svg'
  },
  // [EthChainId.FANTOM]: {
  //   name: 'Fantom Opera',
  //   slug: 'fantom',
  //   chainId: EthChainId.FANTOM,
  //   variation: EthVariation.DEFAULT,
  //   priceTokenAddress: '0x21be370d5312f44cb42ce377bc9b8a0cef1a4c83', // WFTM
  //   tokenAddress: '0x0000000000000000000000000000000000000000',
  //   wrappedTokenAddress: '0x21be370d5312f44cb42ce377bc9b8a0cef1a4c83',
  //   tokenSymbol: 'WFTM',
  //   tokenDecimals: 18,
  //   explorerUrl: 'https://ftmscan.com',
  //   explorerApiType: ExplorerApiType.ETHERSCAN,
  //   explorerApi: 'https://api.ftmscan.com',
  //   lightIcon: 'https://assets.sentio.xyz/chains/fantom.svg'
  // },
  [EthChainId.OPTIMISM]: {
    name: 'Optimism Mainnet',
    slug: 'optimism',
    chainId: EthChainId.OPTIMISM,
    nativeChainId: 10,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://optimistic.etherscan.io',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/optimism.svg'
  },
  [EthChainId.CRONOS]: {
    name: 'Cronos Mainnet',
    slug: 'cronos',
    chainId: EthChainId.CRONOS,
    nativeChainId: 25,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x5c7f8a570d578ed84e63fdfa7b1ee72deae1ae23', // WCRO
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x5c7f8a570d578ed84e63fdfa7b1ee72deae1ae23',
    tokenSymbol: 'CRO',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.cronos.org',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/cronos.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/cronos_light.svg'
  },
  [EthChainId.CRONOS_TESTNET]: {
    name: 'Cronos Testnet',
    slug: 'cronos-testnet',
    chainId: EthChainId.CRONOS_TESTNET,
    nativeChainId: 338,
    mainnetChainId: EthChainId.CRONOS,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x6a3173618859c7cd40faf6921b5e9eb6a76f1fd4', // WCRO
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x6a3173618859c7cd40faf6921b5e9eb6a76f1fd4',
    tokenSymbol: 'CRO',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.cronos.org/testnet',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/cronos.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/cronos_light.svg'
  },
  [EthChainId.BITLAYER]: {
    name: 'Bitlayer Mainnet',
    slug: 'bitlayer',
    chainId: EthChainId.BITLAYER,
    nativeChainId: 200901,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0xff204e2681a6fa0e2c3fade68a1b28fb90e4fc5f', // WBTC
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xff204e2681a6fa0e2c3fade68a1b28fb90e4fc5f',
    tokenSymbol: 'BTC',
    tokenDecimals: 18,
    explorerUrl: 'https://www.btrscan.com',
    explorerApiType: ExplorerApiType.ETHERSCAN,
    explorerApi: 'https://api.btrscan.com/scan',
    lightIcon: 'https://assets.sentio.xyz/chains/bitlayer.svg'
  },
  [EthChainId.MANTA_PACIFIC]: {
    name: 'Manta Pacific',
    slug: 'manta-pacific-mainnet',
    chainId: EthChainId.MANTA_PACIFIC,
    nativeChainId: 169,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x0Dc808adcE2099A9F62AA87D9670745AbA741746',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://pacific-explorer.manta.network',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://pacific-explorer.manta.network',
    lightIcon: 'https://assets.sentio.xyz/chains/manta.png'
  },
  [EthChainId.MANTLE]: {
    name: 'Mantle',
    slug: 'mantle',
    chainId: EthChainId.MANTLE,
    nativeChainId: 5000,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x78c1b0c915c4faa5fffa6cabf0219da63d7f4cb8', // WMNT
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x78c1b0c915c4faa5fffa6cabf0219da63d7f4cb8',
    tokenSymbol: 'MNT',
    tokenDecimals: 18,
    explorerUrl: 'https://mantlescan.xyz',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/mantle.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/mantle-white.svg'
  },
  [EthChainId.B2_MAINNET]: {
    name: 'B2 Mainnet',
    slug: 'b2-mainnet',
    chainId: EthChainId.B2_MAINNET,
    nativeChainId: 223,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x4200000000000000000000000000000000000006', // WBTC
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenSymbol: 'BTC',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.bsquared.network',
    explorerApiType: ExplorerApiType.L2_SCAN,
    explorerApi: 'https://explorer.bsquared.network/api',
    lightIcon: 'https://assets.sentio.xyz/chains/b2.svg'
  },
  [EthChainId.MODE]: {
    name: 'Mode Mainnet',
    slug: 'mode-mainnet',
    chainId: EthChainId.MODE,
    nativeChainId: 34443,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://modescan.io',
    explorerApiType: ExplorerApiType.ETHERSCAN,
    explorerApi:
      'https://api.routescan.io/v2/network/mainnet/evm/34443/etherscan',
    lightIcon: 'https://assets.sentio.xyz/chains/mode.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/mode-dark.svg'
  },
  [EthChainId.BOB]: {
    name: 'Bob Mainnet',
    slug: 'bob',
    chainId: EthChainId.BOB,
    nativeChainId: 60808,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.gobob.xyz',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://explorer.gobob.xyz',
    lightIcon: 'https://assets.sentio.xyz/chains/bob.svg'
  },
  [EthChainId.FRAXTAL]: {
    name: 'Fraxtal Mainnet',
    slug: 'frax-mainnet',
    chainId: EthChainId.FRAXTAL,
    nativeChainId: 252,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x0000000000000000000000000000000000000000', // wfrxETH
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xFC00000000000000000000000000000000000006',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://fraxscan.com',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/fraxtal.svg'
  },
  [EthChainId.GOAT_MAINNET]: {
    name: 'Goat Network',
    slug: 'goat',
    chainId: EthChainId.GOAT_MAINNET,
    nativeChainId: 2345,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xbC10000000000000000000000000000000000000',
    tokenSymbol: 'BTC',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.goat.network',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    lightIcon: 'https://assets.sentio.xyz/chains/goat.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/goat-dark.svg'
  },
  [EthChainId.KUCOIN]: {
    name: 'KCC Mainnet',
    slug: 'kucoin',
    chainId: EthChainId.KUCOIN,
    nativeChainId: 321,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x4446Fc4eb47f2f6586f9fAAb68B3498F86C07521', // WCCS
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4446Fc4eb47f2f6586f9fAAb68B3498F86C07521',
    tokenSymbol: 'KCS',
    tokenDecimals: 18,
    explorerUrl: 'https://scan.kcc.io',
    explorerApiType: ExplorerApiType.ETHERSCAN,
    explorerApi: 'https://scan.kcc.io',
    lightIcon: 'https://assets.sentio.xyz/chains/kcc.svg'
  },
  [EthChainId.CONFLUX]: {
    name: 'Conflux eSpace',
    slug: 'conflux-espace',
    chainId: EthChainId.CONFLUX,
    nativeChainId: 1030,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x14b2d3bc65e74dae1030eafd8ac30c533c976a9b', // WCFX
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x14b2d3bc65e74dae1030eafd8ac30c533c976a9b',
    tokenSymbol: 'CFX',
    tokenDecimals: 18,
    explorerUrl: 'https://evm.confluxscan.io',
    explorerApiType: ExplorerApiType.ETHERSCAN,
    explorerApi: 'https://evmapi.confluxscan.io',
    lightIcon: 'https://assets.sentio.xyz/chains/conflux.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/conflux-white.svg'
  },
  [EthChainId.METIS]: {
    name: 'Metis',
    slug: 'metis',
    chainId: EthChainId.METIS,
    nativeChainId: 1088,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x75cb093E4D61d2A2e65D8e0BBb01DE8d89b53481', // WMETIS
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x75cb093E4D61d2A2e65D8e0BBb01DE8d89b53481',
    tokenSymbol: 'METIS',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.metis.io',
    explorerApiType: ExplorerApiType.ETHERSCAN,
    explorerApi:
      'https://api.routescan.io/v2/network/mainnet/evm/1088/etherscan/',
    lightIcon: 'https://assets.sentio.xyz/chains/metis.svg'
  },
  [EthChainId.BEVM]: {
    name: 'BEVM',
    slug: 'bevm',
    chainId: EthChainId.BEVM,
    nativeChainId: 11501,
    variation: EthVariation.SUBSTRATE,
    priceTokenAddress: '0xB5136FEba197f5fF4B765E5b50c74db717796dcD', // WBTC
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xB5136FEba197f5fF4B765E5b50c74db717796dcD',
    tokenSymbol: 'BTC',
    tokenDecimals: 18,
    explorerUrl: 'https://scan.bevm.io',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://scan.bevm.io',
    lightIcon: 'https://assets.sentio.xyz/chains/bevm.svg'
  },
  [EthChainId.MERLIN_MAINNET]: {
    name: 'Merlin Mainnet',
    slug: 'merlin',
    chainId: EthChainId.MERLIN_MAINNET,
    nativeChainId: 4200,
    variation: EthVariation.POLYGON_ZKEVM,
    priceTokenAddress: '0xF6D226f9Dc15d9bB51182815b320D3fBE324e1bA', // WBTC
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xF6D226f9Dc15d9bB51182815b320D3fBE324e1bA',
    tokenSymbol: 'BTC',
    tokenDecimals: 18,
    explorerUrl: 'https://scan.merlinchain.io',
    explorerApiType: ExplorerApiType.L2_SCAN,
    explorerApi: 'https://scan.merlinchain.io/api',
    lightIcon: 'https://assets.sentio.xyz/chains/merlin.png'
  },
  [EthChainId.CHILIZ]: {
    name: 'Chiliz',
    slug: 'chiliz',
    chainId: EthChainId.CHILIZ,
    nativeChainId: 88888,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x721EF6871f1c4Efe730Dce047D40D1743B886946',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x721EF6871f1c4Efe730Dce047D40D1743B886946', // WCHZ
    tokenSymbol: 'CHZ',
    tokenDecimals: 18,
    explorerUrl: 'https://chiliscan.com',
    explorerApiType: ExplorerApiType.ETHERSCAN,
    explorerApi:
      'https://api.routescan.io/v2/network/mainnet/evm/88888/etherscan',
    lightIcon: 'https://assets.sentio.xyz/chains/chiliz.svg'
  },
  [EthChainId.ZKLINK_NOVA]: {
    name: 'zkLink Nova',
    slug: 'zklink-nova',
    chainId: EthChainId.ZKLINK_NOVA,
    nativeChainId: 810180,
    variation: EthVariation.ZKSYNC,
    priceTokenAddress: '0x000000000000000000000000000000000000800A',
    tokenAddress: '0x000000000000000000000000000000000000800A', //special
    wrappedTokenAddress: '0x000000000000000000000000000000000000800A',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.zklink.io',
    explorerApiType: ExplorerApiType.ETHERSCAN,
    explorerApi: 'https://explorer-api.zklink.io',
    lightIcon: 'https://assets.sentio.xyz/chains/zklink.svg'
  },
  [EthChainId.AURORA]: {
    name: 'Aurora',
    slug: 'aurora',
    chainId: EthChainId.AURORA,
    nativeChainId: 1313161554,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0xC9BdeEd33CD01541e1eeD10f90519d2C06Fe3feB',
    tokenAddress: '0x000000000000000000000000000000000000800A',
    wrappedTokenAddress: '0xC9BdeEd33CD01541e1eeD10f90519d2C06Fe3feB',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.aurora.dev',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://explorer.aurora.dev',
    lightIcon: 'https://assets.sentio.xyz/chains/aurora.svg'
  },
  [EthChainId.SONIC_MAINNET]: {
    name: 'Sonic Mainnet',
    slug: 'sonic-mainnet',
    chainId: EthChainId.SONIC_MAINNET,
    nativeChainId: 146,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x000000000000000000000000000000000000800A',
    tokenSymbol: 'S',
    tokenDecimals: 18,
    explorerUrl: 'https://sonicscan.org',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/sonic.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/sonic-dark.svg'
  },
  [EthChainId.SONIC_TESTNET]: {
    name: 'Sonic Testnet',
    slug: 'sonic-testnet',
    chainId: EthChainId.SONIC_TESTNET,
    nativeChainId: 14601,
    mainnetChainId: EthChainId.SONIC_MAINNET,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x000000000000000000000000000000000000800A',
    tokenSymbol: 'S',
    tokenDecimals: 18,
    explorerUrl: 'https://testnet.sonicscan.org',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerApi: 'https://api.etherscan.io/v2',
    lightIcon: 'https://assets.sentio.xyz/chains/sonic.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/sonic-dark.svg'
  },
  [EthChainId.SONEIUM_MAINNET]: {
    name: 'Soneium Mainnet',
    slug: 'soneium-mainnet',
    chainId: EthChainId.SONEIUM_MAINNET,
    nativeChainId: 1868,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://soneium.blockscout.com',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://soneium.blockscout.com',
    lightIcon: 'https://assets.sentio.xyz/chains/soneium.svg'
  },
  [EthChainId.SONEIUM_TESTNET]: {
    name: 'Soneium Testnet',
    slug: 'soneium-minato',
    chainId: EthChainId.SONEIUM_TESTNET,
    nativeChainId: 1946,
    mainnetChainId: EthChainId.SONEIUM_MAINNET,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x728F2745410A56620B50a6E0592743450e08Cac6',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://soneium-minato.blockscout.com',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://soneium-minato.blockscout.com',
    lightIcon: 'https://assets.sentio.xyz/chains/soneium.svg'
  },
  [EthChainId.CRONOS_ZKEVM]: {
    name: 'Cronos zkEVM',
    slug: 'cronos-zkevm',
    chainId: EthChainId.CRONOS_ZKEVM,
    nativeChainId: 388,
    variation: EthVariation.ZKSYNC,
    priceTokenAddress: '0x000000000000000000000000000000000000800a',
    tokenAddress: '0x000000000000000000000000000000000000800a',
    wrappedTokenAddress: '0xc1bf55ee54e16229d9b369a5502bfe5fc9f20b6d',
    tokenSymbol: 'zkCRO',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.zkevm.cronos.org',
    explorerApiType: ExplorerApiType.UNKNOWN,
    // explorerApi: 'https://explorer.zkevm.cronos.org',
    lightIcon: 'https://assets.sentio.xyz/chains/cronos.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/cronos_light.svg'
  },
  [EthChainId.DERIVE]: {
    name: 'Derive Mainnet',
    slug: 'derive-mainnet',
    chainId: EthChainId.DERIVE,
    nativeChainId: 957,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x15CEcd5190A43C7798dD2058308781D0662e678E',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.lyra.finance',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://explorer.lyra.finance',
    lightIcon: 'https://assets.sentio.xyz/chains/derive.svg'
  },
  [EthChainId.UNICHAIN_SEPOLIA]: {
    name: 'Unichain Sepolia',
    slug: 'unichain-sepolia',
    chainId: EthChainId.UNICHAIN_SEPOLIA,
    nativeChainId: 1301,
    mainnetChainId: EthChainId.UNICHAIN,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://unichain-sepolia.blockscout.com',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://unichain-sepolia.blockscout.com',
    lightIcon: 'https://assets.sentio.xyz/chains/unichain-testnet.svg'
  },
  [EthChainId.UNICHAIN]: {
    name: 'Unichain',
    slug: 'unichain-mainnet',
    chainId: EthChainId.UNICHAIN,
    nativeChainId: 130,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://unichain.blockscout.com',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://unichain.blockscout.com',
    lightIcon: 'https://assets.sentio.xyz/chains/unichain.svg'
  },
  [EthChainId.CORN_MAIZENET]: {
    name: 'Corn Maizenet',
    slug: 'corn-maizenet',
    chainId: EthChainId.CORN_MAIZENET,
    nativeChainId: 21000000,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenSymbol: 'BTCN',
    tokenDecimals: 18,
    explorerUrl: 'https://maizenet-explorer.usecorn.com',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://maizenet-explorer.usecorn.com',
    lightIcon: 'https://assets.sentio.xyz/chains/corn.svg'
  },
  [EthChainId.KARAK]: {
    name: 'Karak Mainnet',
    slug: 'karak-mainnet',
    chainId: EthChainId.KARAK,
    nativeChainId: 2410,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x4200000000000000000000000000000000000006',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.karak.network',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://explorer.karak.network',
    lightIcon: 'https://assets.sentio.xyz/chains/karak.svg'
  },
  [EthChainId.SEI]: {
    name: 'Sei Mainnet',
    slug: 'sei',
    chainId: EthChainId.SEI,
    nativeChainId: 1329,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://seistream.app',
    lightIcon: 'https://assets.sentio.xyz/chains/sei.svg'
  },
  [EthChainId.SWELL_MAINNET]: {
    name: 'Swell Mainnet',
    slug: 'swell-mainnet',
    chainId: EthChainId.SWELL_MAINNET,
    nativeChainId: 1923,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.swellnetwork.io',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://explorer.swellnetwork.io',
    lightIcon: 'https://assets.sentio.xyz/chains/swell.svg'
  },
  [EthChainId.SWELL_TESTNET]: {
    name: 'Swell Testnet',
    slug: 'swell-testnet',
    chainId: EthChainId.SWELL_TESTNET,
    nativeChainId: 1924,
    mainnetChainId: EthChainId.SWELL_MAINNET,
    variation: EthVariation.OPTIMISM,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://swell-testnet-explorer.alt.technology',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://swell-testnet-explorer.alt.technology',
    lightIcon: 'https://assets.sentio.xyz/chains/swell.svg'
  },
  [EthChainId.TAC_TESTNET]: {
    name: 'TAC Testnet',
    slug: 'tac-testnet',
    chainId: EthChainId.TAC_TESTNET,
    nativeChainId: 2390,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x392D1cCB04d25fCBcA7D4fc0E429Dbc1F9fEe73F',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x392D1cCB04d25fCBcA7D4fc0E429Dbc1F9fEe73F',
    tokenSymbol: 'TAC',
    tokenDecimals: 18,
    explorerUrl: 'https://turin.explorer.tac.build',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://turin.explorer.tac.build',
    lightIcon: 'https://assets.sentio.xyz/chains/tac.svg'
  },
  [EthChainId.MONAD_TESTNET]: {
    name: 'Monad Testnet',
    slug: 'monad-testnet',
    chainId: EthChainId.MONAD_TESTNET,
    nativeChainId: 10143,
    mainnetChainId: EthChainId.MONAD_MAINNET,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x760AfE86e5de5fa0Ee542fc7B7B713e1c5425701',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x760AfE86e5de5fa0Ee542fc7B7B713e1c5425701',
    tokenSymbol: 'MON',
    tokenDecimals: 18,
    explorerUrl: 'https://testnet.monadexplorer.com',
    explorerApiType: ExplorerApiType.UNKNOWN,
    lightIcon: 'https://assets.sentio.xyz/chains/monad.svg'
  },
  [EthChainId.MONAD_MAINNET]: {
    name: 'Monad Mainnet',
    slug: 'monad-mainnet',
    chainId: EthChainId.MONAD_MAINNET,
    nativeChainId: 143,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x760AfE86e5de5fa0Ee542fc7B7B713e1c5425701',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x760AfE86e5de5fa0Ee542fc7B7B713e1c5425701',
    tokenSymbol: 'MON',
    tokenDecimals: 18,
    explorerUrl: 'https://monadexplorer.com',
    explorerApiType: ExplorerApiType.UNKNOWN,
    lightIcon: 'https://assets.sentio.xyz/chains/monad.svg'
  },
  [EthChainId.BERACHAIN]: {
    name: 'Berachain',
    slug: 'berachain',
    chainId: EthChainId.BERACHAIN,
    nativeChainId: 80094,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenSymbol: 'BERA',
    tokenDecimals: 18,
    explorerUrl: 'https://berascan.com',
    explorerApi: 'https://api.etherscan.io/v2',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    lightIcon: 'https://assets.sentio.xyz/chains/berachain.svg'
  },
  [EthChainId.HYPER_EVM]: {
    name: 'HyperEVM',
    slug: 'hyperevm',
    additionalSlugs: ['hyper-evm'],
    chainId: EthChainId.HYPER_EVM,
    nativeChainId: 999,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenSymbol: 'HYPE',
    tokenDecimals: 18,
    explorerUrl: 'https://hyperevmscan.io',
    explorerApi: 'https://hyperevmscan.io/v2',
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    lightIcon: 'https://assets.sentio.xyz/chains/hype.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/hype-dark.svg'
  },
  [EthChainId.ETHERLINK]: {
    name: 'Etherlink',
    slug: 'etherlink',
    chainId: EthChainId.ETHERLINK,
    nativeChainId: 42793,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0xc9B53AB2679f573e480d01e0f49e2B5CFB7a3EAb',
    tokenSymbol: 'XTZ',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.etherlink.com',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerApi: 'https://explorer.etherlink.com',
    lightIcon: 'https://assets.sentio.xyz/chains/etherlink.svg'
  },
  [EthChainId.MEV_COMMIT]: {
    name: 'MEV Commit',
    slug: 'mev-commit',
    chainId: EthChainId.MEV_COMMIT,
    nativeChainId: 57173,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://www.mev-commit.xyz',
    lightIcon: 'https://assets.sentio.xyz/chains/mev-commit-dark.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/mev-commit.svg'
  },
  [EthChainId.HEMI]: {
    name: 'Hemi',
    slug: 'hemi',
    chainId: EthChainId.HEMI,
    nativeChainId: 43111,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    explorerUrl: 'https://explorer.hemi.xyz',
    explorerApi: 'https://explorer.hemi.xyz',
    lightIcon: 'https://assets.sentio.xyz/chains/hemi.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/hemi.svg'
  },
  [EthChainId.ABSTRACT]: {
    name: 'Abstract',
    slug: 'abstract',
    chainId: EthChainId.ABSTRACT,
    nativeChainId: 2741,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x000000000000000000000000000000000000800A',
    tokenAddress: '0x000000000000000000000000000000000000800A',
    wrappedTokenAddress: '0x000000000000000000000000000000000000800A',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: 'https://explorer.mainnet.abs.xyz',
    lightIcon: 'https://assets.sentio.xyz/chains/abstract.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/abstract.svg'
  },
  [EthChainId.PLASMA_MAINNET]: {
    name: 'Plasma Mainnet',
    slug: 'plasma-mainnet',
    chainId: EthChainId.PLASMA_MAINNET,
    nativeChainId: 9745,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x6100E367285b01F48D07953803A2d8dCA5D19873',
    tokenSymbol: 'XPL',
    tokenDecimals: 18,
    explorerUrl: 'https://plasmascan.to',
    explorerApi:
      'https://api.routescan.io/v2/network/mainnet/evm/9745/etherscan',
    explorerApiType: ExplorerApiType.ETHERSCAN,
    lightIcon: 'https://assets.sentio.xyz/chains/plasma.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/plasma-dark.svg'
  },
  [EthChainId.PLASMA_TESTNET]: {
    name: 'Plasma Testnet',
    slug: 'plasma-testnet',
    chainId: EthChainId.PLASMA_TESTNET,
    nativeChainId: 9746,
    mainnetChainId: EthChainId.PLASMA_MAINNET,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x6100E367285b01F48D07953803A2d8dCA5D19873',
    tokenSymbol: 'XPL',
    tokenDecimals: 18,
    explorerUrl: 'https://testnet.plasmascan.to',
    explorerApi:
      'https://api.routescan.io/v2/network/mainnet/evm/9746/etherscan',
    explorerApiType: ExplorerApiType.ETHERSCAN,
    lightIcon: 'https://assets.sentio.xyz/chains/plasma.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/plasma-dark.svg'
  },
  [EthChainId.ARC_TESTNET]: {
    name: 'Arc Testnet',
    slug: 'arc-testnet',
    chainId: EthChainId.ARC_TESTNET,
    nativeChainId: 5042002,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x3600000000000000000000000000000000000000',
    tokenAddress: '0x3600000000000000000000000000000000000000',
    wrappedTokenAddress: '0x3600000000000000000000000000000000000000',
    tokenSymbol: 'USDC',
    tokenDecimals: 6,
    explorerUrl: 'https://testnet.arcscan.app',
    explorerApi: 'https://testnet.arcscan.app',
    explorerApiType: ExplorerApiType.BLOCKSCOUT,
    lightIcon: 'https://assets.sentio.xyz/chains/arc.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/arc-dark.svg'
  },
  [EthChainId.STABLE_TESTNET]: {
    name: 'Stable Testnet',
    slug: 'stable-testnet',
    chainId: EthChainId.STABLE_TESTNET,
    nativeChainId: 2201,
    mainnetChainId: EthChainId.STABLE_MAINNET,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000001000',
    tokenAddress: '0x0000000000000000000000000000000000001000',
    wrappedTokenAddress: '0x0000000000000000000000000000000000001000',
    tokenSymbol: 'gUSDT',
    tokenDecimals: 18,
    explorerUrl: 'https://testnet.stablescan.xyz',
    lightIcon: 'https://assets.sentio.xyz/chains/stable.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/stable-dark.svg'
  },
  [EthChainId.STABLE_MAINNET]: {
    name: 'Stable Mainnet',
    slug: 'stable-mainnet',
    chainId: EthChainId.STABLE_MAINNET,
    nativeChainId: 988,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000001000',
    tokenAddress: '0x0000000000000000000000000000000000001000',
    wrappedTokenAddress: '0x0000000000000000000000000000000000001000',
    tokenSymbol: 'gUSDT',
    tokenDecimals: 18,
    explorerUrl: 'https://stablescan.xyz',
    lightIcon: 'https://assets.sentio.xyz/chains/stable.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/stable-dark.svg'
  },
  [EthChainId.SENTIO_MAINNET]: {
    name: 'Sentio mainnet',
    slug: 'sentio-mainnet',
    chainId: EthChainId.SENTIO_MAINNET,
    nativeChainId: 789210,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x1ef5f52bdbe11af2377c58ecc914a8c72ea807cf',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerUrl: 'https://sepolia.etherscan.io',
    explorerApi: 'https://api.etherscan.io/v2',
    blockscoutUrl: 'https://eth-sepolia.blockscout.com',
    lightIcon: 'https://assets.sentio.xyz/chains/eth.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/eth-dark.svg'
  },
  [EthChainId.SENTIO_TESTNET]: {
    name: 'Sentio testnet',
    slug: 'sentio-testnet',
    chainId: EthChainId.SENTIO_TESTNET,
    nativeChainId: 7892101,
    mainnetChainId: EthChainId.SENTIO_MAINNET,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x1ef5f52bdbe11af2377c58ecc914a8c72ea807cf',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerApiType: ExplorerApiType.ETHERSCAN_V2,
    explorerUrl: 'https://sepolia.etherscan.io',
    explorerApi: 'https://api.etherscan.io/v2',
    blockscoutUrl: 'https://eth-sepolia.blockscout.com',
    lightIcon: 'https://assets.sentio.xyz/chains/eth.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/eth-dark.svg'
  },
  [EthChainId.CUSTOM]: {
    name: 'Custom',
    slug: 'custom',
    chainId: EthChainId.CUSTOM,
    nativeChainId: 0,
    variation: EthVariation.DEFAULT,
    priceTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenAddress: '0x0000000000000000000000000000000000000000',
    wrappedTokenAddress: '0x0000000000000000000000000000000000000000',
    tokenSymbol: 'ETH',
    tokenDecimals: 18,
    explorerUrl: '',
    lightIcon: 'https://assets.sentio.xyz/chains/eth.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/eth-dark.svg'
  }
}

type ScanUrlSubType = 'block' | 'address' | 'tx' | 'token' | 'object'
type ScanSubPath = Partial<Record<ScanUrlSubType, string | undefined>>

function getEVMChainScanUrl(
  chainId: string | number,
  hash: string,
  subtype: ScanUrlSubType
) {
  // TODO l2scan address might be different
  let subtypeStr: string = subtype
  const supportedChain = EthChainInfo[chainId as ChainId]
  if (!supportedChain) {
    return
  }
  if (supportedChain.explorerApiType === ExplorerApiType.L2_SCAN) {
    if (subtype === 'block') {
      subtypeStr = 'blocks'
    }
  }
  return `${supportedChain.explorerUrl}/${subtypeStr}/${hash}`
}

/**
 * BTC chains
 */
export const BTCChainInfo: Record<BTCChainId | string, ChainInfo> = {
  [BTCChainId.BTC_MAINNET]: {
    name: 'Bitcoin Mainnet',
    slug: 'btc',
    chainId: BTCChainId.BTC_MAINNET,
    explorerUrl: 'https://mempool.space',
    lightIcon: 'https://assets.sentio.xyz/chains/bitcoin.svg'
  },
  [BTCChainId.BTC_TESTNET]: {
    name: 'Bitcoin Mainnet',
    slug: 'btc-signet',
    chainId: BTCChainId.BTC_TESTNET,
    explorerUrl: 'https://mempool.space/testnet4',
    lightIcon: 'https://assets.sentio.xyz/chains/bitcoin-testnet.svg'
  }
}

const BtcSubTypePaths: ScanSubPath = {
  block: 'block',
  address: 'address',
  tx: 'tx',
  token: undefined
}

function getBtcChainScanUrl(
  chainId: string | number,
  hash: string,
  subtype: ScanUrlSubType
) {
  const hostName = BTCChainInfo[chainId]?.explorerUrl
  const subPath = BtcSubTypePaths[subtype]
  if (!hostName || !subPath) {
    return
  }
  return `${hostName}/${subPath}/${hash}`
}

/**
 * Aptos chains
 */
export const AptosChainInfo: Record<
  AptosChainId | string,
  ChainInfo & {
    suffix: string
  }
> = {
  [AptosChainId.APTOS_MAINNET]: {
    name: 'Aptos Mainnet',
    slug: 'aptos',
    chainId: AptosChainId.APTOS_MAINNET,
    nativeChainId: 1,
    explorerUrl: 'https://explorer.aptoslabs.com',
    suffix: '?network=mainnet',
    lightIcon: 'https://assets.sentio.xyz/chains/aptos.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/aptos-dark.svg'
  },
  [AptosChainId.APTOS_TESTNET]: {
    name: 'Aptos Testnet',
    chainId: AptosChainId.APTOS_TESTNET,
    nativeChainId: 2,
    mainnetChainId: AptosChainId.APTOS_MAINNET,
    slug: 'aptos-testnet',
    explorerUrl: 'https://explorer.aptoslabs.com',
    suffix: '?network=testnet',
    lightIcon: 'https://assets.sentio.xyz/chains/aptos.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/aptos-dark.svg'
  },
  [AptosChainId.APTOS_MOVEMENT_MAINNET]: {
    name: 'Movement Mainnet',
    slug: 'movement',
    chainId: AptosChainId.APTOS_MOVEMENT_MAINNET,
    nativeChainId: 126,
    explorerUrl: 'https://explorer.movementnetwork.xyz',
    suffix: '?network=mainnet',
    lightIcon: 'https://assets.sentio.xyz/chains/movement.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/movement-dark.svg'
  },
  [AptosChainId.APTOS_MOVEMENT_TESTNET]: {
    name: 'Movement Testnet',
    slug: 'movement-testnet',
    chainId: AptosChainId.APTOS_MOVEMENT_TESTNET,
    nativeChainId: 250,
    mainnetChainId: AptosChainId.APTOS_MOVEMENT_MAINNET,
    explorerUrl: 'https://explorer.movementnetwork.xyz',
    suffix: '?network=testnet',
    lightIcon: 'https://assets.sentio.xyz/chains/movement.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/movement-dark.svg'
  },
  [AptosChainId.APTOS_MOVEMENT_PREVIEWNET]: {
    name: 'Movement Previewnet',
    slug: 'movement-previewnet',
    chainId: AptosChainId.APTOS_MOVEMENT_PREVIEWNET,
    mainnetChainId: AptosChainId.APTOS_MOVEMENT_MAINNET,
    explorerUrl: 'https://explorer.movementnetwork.xyz',
    suffix: '?network=previewnet',
    lightIcon: 'https://assets.sentio.xyz/chains/movement.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/movement-dark.svg'
  },
  [AptosChainId.INITIA_ECHELON]: {
    name: 'Initia Echelon',
    slug: 'initia-echelon',
    chainId: AptosChainId.INITIA_ECHELON,
    explorerUrl: 'https://scan.initia.xyz/echelon-1',
    suffix: '',
    lightIcon: 'https://assets.sentio.xyz/chains/initia-echelon.svg'
  }
}
const AptosSubTypePaths: ScanSubPath = {
  block: 'block',
  address: 'account',
  tx: 'txn',
  token: undefined
}
function getAptosChainScanUrl(
  chainId: string | number,
  hash: string,
  subtype: ScanUrlSubType
) {
  const { explorerUrl, suffix } = AptosChainInfo[chainId]
  const subPath = AptosSubTypePaths[subtype]
  if (!subPath) {
    return
  }
  return `${explorerUrl}/${subPath}/${hash}${suffix}`
}

/**
 * Solana
 */
export const SolanaChainInfo: Record<
  SolanaChainId | string,
  ChainInfo & {
    suffix: string
  }
> = {
  [SolanaChainId.SOLANA_MAINNET]: {
    name: 'Solana Mainnet',
    slug: 'solana',
    chainId: SolanaChainId.SOLANA_MAINNET,
    nativeChainId: 101,
    explorerUrl: 'https://solscan.io/',
    suffix: '',
    lightIcon: 'https://sentio.xyz/solana.svg'
  },
  [SolanaChainId.SOLANA_TESTNET]: {
    name: 'Solana Testnet',
    slug: 'solana-testnet',
    chainId: SolanaChainId.SOLANA_TESTNET,
    nativeChainId: 102,
    mainnetChainId: SolanaChainId.SOLANA_MAINNET,
    explorerUrl: 'https://solscan.io/',
    suffix: '?cluster=testnet',
    lightIcon: 'https://sentio.xyz/solana.svg'
  },
  [SolanaChainId.SOLANA_PYTH]: {
    name: 'Pyth',
    slug: 'pyth',
    chainId: SolanaChainId.SOLANA_PYTH,
    nativeChainId: 101,
    explorerUrl: 'https://solscan.io/',
    suffix: '?cluster=custom&customUrl=https://pythnet.rpcpool.com',
    lightIcon: 'https://sentio.xyz/pyth.svg'
  },
  [SolanaChainId.FORGO_TESTNET]: {
    name: 'Forgo Testnet',
    slug: 'forgo-testnet',
    chainId: SolanaChainId.FORGO_TESTNET,
    mainnetChainId: SolanaChainId.FORGO_MAINNET,
    explorerUrl: 'https://fogoscan.com/',
    suffix: '?cluster=testnet',
    lightIcon: 'https://sentio.xyz/solana.svg'
  },
  [SolanaChainId.FORGO_MAINNET]: {
    name: 'Forgo Mainnet',
    slug: 'forgo-mainnet',
    chainId: SolanaChainId.FORGO_MAINNET,
    explorerUrl: 'https://fogoscan.com/',
    suffix: '',
    lightIcon: 'https://sentio.xyz/solana.svg'
  }
}

const SolanaSubTypePaths: ScanSubPath = {
  block: 'block',
  address: 'address',
  tx: 'tx',
  token: 'token'
}

function getSolanaChainScanUrl(
  chainId: string | number,
  hash: string,
  subtype: ScanUrlSubType
) {
  const { explorerUrl, suffix } = SolanaChainInfo[chainId]
  const subPath = SolanaSubTypePaths[subtype]
  if (!subPath) {
    return
  }
  return `${explorerUrl}${subPath}/${hash}${suffix}`
}

/**
 * Sui
 */
export const SuiChainInfo: Record<
  SuiChainId | string,
  ChainInfo & {
    suiscanUrl?: string
    suivisionUrl?: string
  }
> = {
  [SuiChainId.SUI_MAINNET]: {
    name: 'Sui Mainnet',
    slug: 'sui',
    chainId: SuiChainId.SUI_MAINNET,
    nativeChainId: 897796746,
    suivisionUrl: 'https://suivision.xyz',
    explorerUrl: 'https://suiscan.xyz/mainnet',
    lightIcon: 'https://assets.sentio.xyz/chains/sui.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/sui-dark.svg'
  },
  [SuiChainId.SUI_TESTNET]: {
    name: 'Sui Testnet',
    slug: 'sui-testnet',
    chainId: SuiChainId.SUI_TESTNET,
    nativeChainId: 1282977196,
    mainnetChainId: SuiChainId.SUI_MAINNET,
    suivisionUrl: 'https://testnet.suivision.xyz',
    explorerUrl: 'https://suiscan.xyz/testnet',
    lightIcon: 'https://assets.sentio.xyz/chains/sui.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/sui-dark.svg'
  },
  [SuiChainId.IOTA_MAINNET]: {
    name: 'IOTA Mainnet',
    slug: 'iota',
    chainId: SuiChainId.IOTA_MAINNET,
    nativeChainId: 1667541717,
    suivisionUrl: '',
    explorerUrl: 'https://iotascan.com/mainnet',
    lightIcon: 'https://assets.sentio.xyz/chains/iota.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/iota-dark.svg'
  },
  [SuiChainId.IOTA_TESTNET]: {
    name: 'IOTA Testnet',
    slug: 'iota-testnet',
    chainId: SuiChainId.IOTA_TESTNET,
    nativeChainId: 587508375,
    mainnetChainId: SuiChainId.IOTA_MAINNET,
    suivisionUrl: '',
    explorerUrl: 'https://iotascan.com/testnet',
    lightIcon: 'https://assets.sentio.xyz/chains/iota.svg',
    darkIcon: 'https://assets.sentio.xyz/chains/iota-dark.svg'
  }
}

const SuiScanSubTypePaths: ScanSubPath = {
  block: 'checkpoint',
  address: 'account',
  tx: 'tx',
  token: 'coin',
  object: 'object'
}

const SuiVisionSubTypePaths: ScanSubPath = {
  block: 'checkpoint',
  address: 'account',
  tx: 'txblock',
  token: 'coin',
  object: 'object'
}

function getSuiChainScanUrl(
  chainId: string | number,
  hash: string,
  subtype: ScanUrlSubType
) {
  const suiChain = SuiChainInfo[chainId]
  if (!suiChain) {
    return
  }
  if (!suiChain.explorerUrl) {
    return
  }
  const subPath = SuiScanSubTypePaths[subtype]
  if (!subPath) {
    return
  }
  return `${suiChain.explorerUrl}/${subPath}/${hash}`
}

function getSuiChainVisionUrl(
  chainId: string | number,
  hash: string,
  subtype: ScanUrlSubType
) {
  const suiChain = SuiChainInfo[chainId]
  if (!suiChain) {
    return
  }
  if (!suiChain.suivisionUrl) {
    return
  }
  const subPath = SuiVisionSubTypePaths[subtype]
  if (!subPath) {
    return
  }
  return `${suiChain.suivisionUrl}/${subPath}/${hash}`
}

/**
 * Fuel
 */
export const FuelChainInfo: Record<FuelChainId | string, ChainInfo> = {
  [FuelChainId.FUEL_MAINNET]: {
    name: 'Fuel Mainnet',
    slug: 'fuel',
    chainId: FuelChainId.FUEL_MAINNET,
    nativeChainId: 9889,
    explorerUrl: 'https://app.fuel.network',
    lightIcon: 'https://assets.sentio.xyz/chains/fuel.svg'
  },
  [FuelChainId.FUEL_TESTNET]: {
    name: 'Fuel Testnet',
    slug: 'fuel-testnet',
    chainId: FuelChainId.FUEL_TESTNET,
    mainnetChainId: FuelChainId.FUEL_MAINNET,
    explorerUrl: 'https://app-testnet.fuel.network',
    lightIcon: 'https://assets.sentio.xyz/chains/fuel.svg'
  }
}

export const StarknetChainInfo: Record<StarknetChainId | string, ChainInfo> = {
  [StarknetChainId.STARKNET_MAINNET]: {
    name: 'Starknet',
    slug: 'starknet',
    chainId: StarknetChainId.STARKNET_MAINNET,
    explorerUrl: 'https://starkscan.co',
    lightIcon: 'https://assets.sentio.xyz/chains/starknet.svg'
  },
  [StarknetChainId.STARKNET_SEPOLIA]: {
    name: 'Starknet Sepolia',
    slug: 'starknet-sepolia',
    chainId: StarknetChainId.STARKNET_SEPOLIA,
    explorerUrl: 'https://sepolia.starkscan.co',
    lightIcon: 'https://assets.sentio.xyz/chains/starknet.svg'
  }
}

export const CosmosChainInfo: Record<CosmosChainId | string, ChainInfo> = {
  [CosmosChainId.INJECTIVE_MAINNET]: {
    name: 'Injective',
    slug: 'injective',
    chainId: CosmosChainId.INJECTIVE_MAINNET,
    explorerUrl: 'https://injscan.com/',
    lightIcon: 'https://assets.sentio.xyz/chains/injective.svg'
  },
  [CosmosChainId.INJECTIVE_TESTNET]: {
    name: 'Injective Testnet',
    slug: 'injective-testnet',
    chainId: CosmosChainId.INJECTIVE_TESTNET,
    mainnetChainId: CosmosChainId.INJECTIVE_MAINNET,
    explorerUrl: 'https://testnet.explorer.injective.network',
    lightIcon: 'https://assets.sentio.xyz/chains/injective.svg'
  }
}

export const NonEthChainInfo: Record<ChainId | string, ChainInfo> = {
  ...BTCChainInfo,
  ...AptosChainInfo,
  ...SolanaChainInfo,
  ...SuiChainInfo,
  ...FuelChainInfo,
  ...StarknetChainInfo,
  ...CosmosChainInfo
}

export const ChainInfo: Record<ChainId | string, ChainInfo> = {
  ...EthChainInfo,
  ...NonEthChainInfo
}

const FuelSubTypePaths: ScanSubPath = {
  block: 'block',
  address: 'account',
  tx: 'tx',
  token: undefined
}

function getFuelChainScanUrl(
  chainId: string | number,
  hash: string,
  subtype: ScanUrlSubType
) {
  const { explorerUrl } = FuelChainInfo[chainId]
  const subPath = FuelSubTypePaths[subtype]
  if (!subPath) {
    return
  }
  return `${explorerUrl}/${subPath}/${hash}`
}

/**
 * Generate scan url of target chain and sub types.
 * @param chainId
 * @param hash
 * @param subtype
 * @returns
 */
export function getChainExternalUrl(
  chainId?: string | number,
  hash?: string,
  subtype?: ScanUrlSubType
): string | undefined {
  if (!chainId || !hash || !subtype) {
    return
  }
  const chainIdStr = chainId.toString()
  if (Object.keys(EthChainInfo).includes(chainIdStr)) {
    // EVM
    return getEVMChainScanUrl(chainIdStr, hash, subtype)
  } else if (Object.keys(BTCChainInfo).includes(chainIdStr)) {
    // BTC
    return getBtcChainScanUrl(chainIdStr, hash, subtype)
  } else if (Object.keys(AptosChainInfo).includes(chainIdStr)) {
    // Aptos
    return getAptosChainScanUrl(chainIdStr, hash, subtype)
  } else if (Object.keys(SolanaChainInfo).includes(chainIdStr)) {
    // Solana
    return getSolanaChainScanUrl(chainIdStr, hash, subtype)
  } else if (Object.keys(SuiChainInfo).includes(chainIdStr)) {
    // Sui
    return (
      getSuiChainVisionUrl(chainIdStr, hash, subtype) ||
      getSuiChainScanUrl(chainIdStr, hash, subtype)
    )
  } else if (Object.keys(FuelChainInfo).includes(chainIdStr)) {
    // Fuel
    return getFuelChainScanUrl(chainIdStr, hash, subtype)
  }
  return
}

export function getChainBlockscoutUrl(
  chainId?: string | number,
  hash?: string,
  subtype?: ScanUrlSubType
): string | undefined {
  if (!chainId || !hash || !subtype) {
    return
  }
  const supportedChain = EthChainInfo[chainId as ChainId]
  if (!supportedChain) {
    return
  }
  if (!supportedChain.blockscoutUrl) {
    return
  }
  return `${supportedChain.blockscoutUrl}/${subtype}/${hash}`
}

export function getSuiscanUrl(
  chainId?: string | number,
  hash?: string,
  subtype?: ScanUrlSubType
) {
  if (!chainId || !hash || !subtype) {
    return
  }
  return getSuiChainScanUrl(chainId, hash, subtype)
}

function getLogoUrl(info?: ChainInfo, dark?: boolean) {
  const defaultUrl = 'https://assets.sentio.xyz/chains/chain-unknown.webp'
  if (!info) {
    return defaultUrl
  }
  if (dark && info?.darkIcon) {
    return info.darkIcon
  }
  if (info?.lightIcon) {
    return info.lightIcon
  }
  return defaultUrl
}

export function getChainLogo(chainId?: string, dark?: boolean) {
  if (!chainId) {
    return
  }
  const chainInfo = ChainInfo[chainId.toString()]
  return getLogoUrl(chainInfo, dark)
}

/**
 * Get the mainnet chain ID for a given chain ID.
 * If the chain is already a mainnet chain or doesn't have a mainnetChainId configured,
 * returns the original chain ID.
 * @param chainId - The chain ID to get the mainnet chain ID for
 * @returns The mainnet chain ID or the original chain ID if no mainnet mapping exists
 */
export function getMainnetChain(chainId: ChainId): ChainId {
  const chainInfo = ChainInfo[chainId.toString()]
  if (!chainInfo) {
    return chainId
  }
  return chainInfo.mainnetChainId || chainId
}
