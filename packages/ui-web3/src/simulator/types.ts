export enum AmountUnit {
  Wei = 'wei',
  Gwei = 'gwei',
  Ether = 'ether'
}

export interface AbiInput {
  name: string
  type: string
  internalType?: string
  components?: AbiInput[]
}

export interface AbiFunction {
  name: string
  type: string
  inputs: AbiInput[]
  outputs?: AbiInput[]
  stateMutability?: string
}

export interface Contract {
  address: string
  chainId: string
  name?: string
  abi?: AbiFunction[]
}

export interface FunctionParam {
  name: string
  type: string
  value: any
}

export interface AccessListItem {
  address: string
  storageKeys: string[]
}

export interface StateOverrideItem {
  contract: string
  balance?: string
  storage?: Array<{
    key: string
    value: string
  }>
}

export interface SourceOverrideItem {
  address: string
  compilationId: string
}

export interface SimulationFormType {
  blockNumber: number
  txIndex: number
  from: string
  to?: string
  gas: number
  gasPrice: number | string
  value: number | string
  header: {
    blockNumber?: number
    blockNumberState?: boolean
    timestamp?: number | string
    timestampState?: boolean
  }
  stateOverride?: StateOverrideItem[]
  sourceOverrides?: SourceOverrideItem[]
  function?: AbiFunction | null
  functionParams?: FunctionParam[]
  input?: string
  contract?: Contract | null
  accessList?: AccessListItem[]
  projectOwner?: string
  projectSlug?: string
  blockDefaultTimestamp?: number
}

export interface Simulation {
  chainSpec?: {
    chainId?: string
    forkId?: string
  }
  blockNumber?: string
  transactionIndex?: string
  from?: string
  to?: string
  value?: string
  gas?: string
  gasPrice?: string
  input?: string
  blockOverride?: {
    blockNumber?: string
    timestamp?: string
  }
  stateOverrides?: Record<
    string,
    {
      balance?: string
      state?: Record<string, string>
    }
  >
  sourceOverrides?: Record<string, string>
  accessList?: AccessListItem[]
  originTxHash?: string
}

export interface SimulateTransactionRequest {
  projectOwner?: string
  projectSlug?: string
  simulation?: Simulation
}

export interface SimulateTransactionResponse {
  simulation?: {
    id?: string
    status?: string
  }
  projectOwner?: string
  projectSlug?: string
}

export type FunctionParamType = {
  internalType: string
  name: string
  type: string
}

export type FunctionType = {
  inputs: FunctionParamType[]
  outputs: FunctionParamType[]
  name: string
  stateMutability: string
  visibility: string
  type: 'function' | 'event'
  anonymous?: boolean
}
