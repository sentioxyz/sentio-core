// copy from https://github.com/DefiLlama/chainlist/blob/main/constants/chainIds.js
// and https://besu.hyperledger.org/en/stable/Concepts/NetworkID-And-ChainID/

export enum ChainType {
  SOLANA = 'solana',
  SUI = 'sui',
  APTOS = 'aptos',
  ETH = 'evm',
  BTC = 'btc',
  COSMOS = 'cosmos',
  STARKNET = 'starknet',
  FUEL = 'fuel'
}

export enum EthChainId {
  // Any modify to this must also be modify in EthChainName
  ETHEREUM = '1',
  OPTIMISM = '10',
  CRONOS = '25',
  BSC = '56',
  BSC_TESTNET = '97',
  UNICHAIN = '130',
  POLYGON = '137',
  MANTA_PACIFIC = '169',
  XLAYER_TESTNET = '195',
  XLAYER_MAINNET = '196',
  OP_BNB_MAINNET = '204',
  SONIC_MAINNET = '146',
  SONIC_TESTNET = '14601',
  B2_MAINNET = '223',
  // FANTOM = '250',
  FRAXTAL = '252',
  KUCOIN = '321',
  CRONOS_ZKEVM = '388',
  ZKSYNC_ERA = '324',
  CRONOS_TESTNET = '338',
  ASTAR = '592',
  DERIVE = '957',
  STABLE_TESTNET = '2201',
  STABLE_MAINNET = '988',
  HYPER_EVM = '999',
  POLYGON_ZKEVM = '1101',
  CONFLUX = '1030',
  METIS = '1088',
  CORE_MAINNET = '1116',
  MOONBEAM = '1284',
  UNICHAIN_SEPOLIA = '1301',
  SEI = '1329',
  SONEIUM_TESTNET = '1946',
  SONEIUM_MAINNET = '1868',
  SWELL_TESTNET = '1924',
  SWELL_MAINNET = '1923',
  TAC_TESTNET = '2390',
  KARAK = '2410',
  BEVM = '11501',
  MERLIN_MAINNET = '4200',
  MANTLE = '5000',
  BASE = '8453',
  BASE_SEPOLIA = '84532',
  MEV_COMMIT = '57173',
  PLASMA_MAINNET = '9745',
  PLASMA_TESTNET = '9746',
  MONAD_TESTNET = '10143',
  MONAD_MAINNET = '143',
  HOLESKY = '17000',
  HOODI = '560048',
  MODE = '34443',
  ARBITRUM = '42161',
  HEMI = '43111',
  AVALANCHE = '43114',
  ZIRCUIT_GARFIELD_TESTNET = '48898',
  ZIRCUIT_MAINNET = '48900',
  BOB = '60808',
  LINEA = '59144',
  BERACHAIN = '80094',
  BLAST = '81457',
  CHILIZ = '88888',
  TAIKO = '167000',
  KATANA_MAINNET = '747474',
  // TAIKO_TESTNET = '167009',
  BITLAYER = '200901',
  SCROLL = '534352',
  ZKLINK_NOVA = '810180',
  SEPOLIA = '11155111',
  ETHERLINK = '42793',
  CORN_MAIZENET = '21000000',
  AURORA = '1313161554',
  BLAST_SEPOLIA = '168587773',
  ABSTRACT = '2741',
  GOAT_MAINNET = '2345',
  ARC_TESTNET = '5042002',
  SENTIO_MAINNET = '789210',
  SENTIO_TESTNET = '7892101',

  CUSTOM = 'customized'
}

export enum AptosChainId {
  APTOS_MAINNET = 'aptos_mainnet',
  APTOS_TESTNET = 'aptos_testnet',
  APTOS_MOVEMENT_TESTNET = 'aptos_movement_testnet',
  APTOS_MOVEMENT_MAINNET = 'aptos_movement_mainnet',
  APTOS_MOVEMENT_PREVIEWNET = 'aptos_movement_previewnet',
  INITIA_ECHELON = 'aptos_echelon'
}

export enum SuiChainId {
  SUI_MAINNET = 'sui_mainnet',
  SUI_TESTNET = 'sui_testnet',
  IOTA_MAINNET = 'iota_mainnet',
  IOTA_TESTNET = 'iota_testnet'
}

export enum SolanaChainId {
  SOLANA_MAINNET = 'sol_mainnet',
  // SOLANA_DEVNET = 'sol_devnet',
  SOLANA_TESTNET = 'sol_testnet',
  SOLANA_PYTH = 'sol_pyth',
  FORGO_TESTNET = 'forgo_testnet',
  FORGO_MAINNET = 'forgo_mainnet'
}
export enum FuelChainId {
  FUEL_MAINNET = 'fuel_mainnet',
  FUEL_TESTNET = 'fuel_testnet'
}

export enum CosmosChainId {
  INJECTIVE_MAINNET = 'injective_mainnet',
  INJECTIVE_TESTNET = 'injective_testnet'
}

export enum StarknetChainId {
  STARKNET_MAINNET = 'starknet_mainnet',
  STARKNET_SEPOLIA = 'starknet_sepolia'
}

export enum BTCChainId {
  BTC_MAINNET = 'btc_mainnet',
  BTC_TESTNET = 'btc_testnet'
}

export type ChainId =
  | EthChainId
  | AptosChainId
  | SuiChainId
  | SolanaChainId
  | FuelChainId
  | CosmosChainId
  | StarknetChainId
  | BTCChainId

export const NonEthChainId = {
  ...AptosChainId,
  ...SuiChainId,
  ...SolanaChainId,
  ...FuelChainId,
  ...CosmosChainId,
  ...StarknetChainId,
  ...BTCChainId
}

export const ChainId = {
  ...EthChainId,
  ...NonEthChainId
}

export const ChainTypeToChainId: Record<ChainType, object> = {
  [ChainType.SOLANA]: SolanaChainId,
  [ChainType.SUI]: SuiChainId,
  [ChainType.COSMOS]: CosmosChainId,
  [ChainType.STARKNET]: StarknetChainId,
  [ChainType.ETH]: EthChainId,
  [ChainType.APTOS]: AptosChainId,
  [ChainType.BTC]: BTCChainId,
  [ChainType.FUEL]: FuelChainId
}

export const ChainIdToType = new Map<string, ChainType>()

for (const [chainType, chainId] of Object.entries(ChainTypeToChainId)) {
  for (const value of Object.values(chainId)) {
    ChainIdToType.set(value, chainType as ChainType)
  }
}

export function getChainType(chainId?: string | number): ChainType {
  const id = String(chainId).toLowerCase()
  const chainType = ChainIdToType.get(id)
  if (!chainType) {
    throw new Error(`Invalid chainType: ${id}`)
  }
  return chainType
}

export function isChainType(
  chainId: string | number,
  targetChainType: ChainType
): boolean {
  const id = String(chainId).toLowerCase()
  const chainType = ChainIdToType.get(id)
  if (!chainType) {
    return false
  }
  return chainType === targetChainType
}
