import { OneOf } from './base'


export type ChainIdentifier = {}
  & OneOf<{ chainId: string; forkId: string }>

export type AccessListItem = {
  address?: string
  storageKeys?: string[]
}

type BaseStateOverride = {
  state?: {[key: string]: string}
}

export type SimulationResult = {
  transaction?: Transaction
  transactionReceipt?: TransactionReceipt
}

export type StateOverride = BaseStateOverride
  & OneOf<{ balance: string }>
  & OneOf<{ code: string }>

export type Transaction = {
  blockNumber?: string
  blockHash?: string
  transactionIndex?: string
  hash?: string
  chainId?: string
  type?: string
  from?: string
  to?: string
  input?: string
  value?: string
  nonce?: string
  gas?: string
  gasPrice?: string
  maxFeePerGas?: string
  maxPriorityFeePerGas?: string
  accessList?: AccessListItem[]
}

export type TransactionReceipt = {
  gasUsed?: string
  cumulativeGasUsed?: string
  effectiveGasPrice?: string
  status?: string
  error?: string
  revertReason?: string
  logs?: any[]
}

type BaseSimulation = {
  id?: string
  createAt?: string
  bundleId?: string
  networkId?: string
  chainId?: string
  chainSpec?: ChainIdentifier
  to?: string
  input?: string
  blockNumber?: string
  transactionIndex?: string
  from?: string
  gas?: string
  gasPrice?: string
  maxFeePerGas?: string
  maxPriorityFeePerGas?: string
  value?: string
  accessList?: AccessListItem[]
  originTxHash?: string
  label?: string
  stateOverrides?: {[key: string]: StateOverride}
  sourceOverrides?: {[key: string]: string}
  debugDeployment?: boolean
  result?: SimulationResult
  sharing?: SimulationSharing
}

type BaseBlockOverrides = {
  blockHash?: {[key: number]: string}
}

export type BlockOverrides = BaseBlockOverrides
  & OneOf<{ blockNumber: string }>
  & OneOf<{ timestamp: string }>
  & OneOf<{ gasLimit: string }>
  & OneOf<{ difficulty: string }>
  & OneOf<{ baseFee: string }>

export type Simulation = BaseSimulation
  & OneOf<{ blockOverride: BlockOverrides }>

export type SimulationSharing = {
  isPublic?: boolean
  id?: string
  simulationId?: string
}