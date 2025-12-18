import { DecodedCallTrace, DecodedLog, LocationWithInstructionIndex } from '@sentio/debugger-common'
import { Breakpoint } from '@sentio/debugger'
// import { Contract } from 'gen/service/common/index'
import * as monaco from 'monaco-editor'
type Contract = any
type DecodedStorage = any

// This address is a placeholder, used to represent native token address
export const NativeTokenAddress = '0x0000000000000000000000000000000000000000'

export interface ExtendedLog extends DecodedLog {
  location: LocationWithInstructionIndex
  // unique id, [index].(c[index]|e[index])+...
  wkey?: string
  // parent continas error
  parentError?: boolean
}

export interface ExtendedCall extends DecodedCallTrace {
  // def location TODO check why need override this
  defLocation?: LocationWithInstructionIndex
  logs: ExtendedLog[]
  calls: ExtendedCall[]
  op?: string // compatible with simulation structure, TODO remove this

  // unique id, [index].(c[index]|e[index])+...
  wkey?: string
  // parent contains error
  parentError?: boolean
  // parent function name
  parentFunctionName?: string
}

export interface ExtendedStorage extends DecodedStorage {
  wkey?: string
}

export type Source = {
  compilationId: string
  address?: string
  filePath: string
}

export const enum BreakpointStatus {
  Disabled = 'disabled',
  Enabled = 'enabled',
  Loading = 'loading',
  Preview = 'preview'
}

export function parseSource(data: string): Source {
  const [compilationId, filePath] = data.split('/').slice(1)
  return { compilationId, filePath }
}

export interface WatchedVariable {
  id: string
  expression?: string
  value?: any
  error?: boolean
}

export interface Transaction {
  accessList: any[]
  blockHash: string
  blockNumber: string
  chainId: string
  from: string
  gas: string
  gasPrice: string
  hash: string
  input: string
  nonce: string
  r: string
  s: string
  to: string
  transactionIndex: string
  type: string
  v: string
  value: string
  maxFeePerGas: string
  maxPriorityFeePerGas: string
}

export interface Block {
  baseFeePerGas: string
  difficulty: string
  extraData: string
  gasLimit: string
  gasUsed: string
  hash: string
  logsBloom: string
  miner: string
  mixHash: string
  nonce: string
  number: string
  parentHash: string
  receiptsRoot: string
  sha3Uncles: string
  size: string
  stateRoot: string
  timestamp: string
  totalDifficulty: string
  transactions: string[]
  transactionsRoot: string
  uncles: string[]
}

export interface TransactionReciept {
  blockHash: string
  blockNumber: string
  contractAddress: string
  cumulativeGasUsed: string
  effectiveGasPrice: string
  from: string
  gasUsed: string
  logs: any[]
  logsBloom: string
  status: string
  to: string
  transactionHash: string
  transactionIndex: string
  type: string
}

export interface StateItem {
  balance?: string
  nonce?: number
  code?: string
  storage?: {
    [key: string]: string
  }
}

export interface StateDiff {
  raw?: RawStateDiff
}

export interface RawStateDiff {
  post: Record<string, StateItem>
  pre: Record<string, StateItem>
}

export interface DecodedStateDiff {
  post: Record<string, DecodedStateItem>
  pre: Record<string, DecodedStateItem>
}

export interface DecodedVariable {
  decoded: any
  id: string
  location: {
    compilationId: string
    lines: {
      start: {
        column: number
        line: number
      }
      end: {
        column: number
        line: number
      }
    }
    sourcePath: string
  }
  type: {
    name: string
    type: string
  }
}

export interface DecodedStateItem {
  balance?: string
  storage?: Record<string, string>
  nonce?: number
  variables?: DecodedVariable[]
}

export type GasInfo = {
  gasUsed: bigint
  gasLimit: bigint
  gasLeft: bigint
  gasCost: bigint
  op: string
}

export type ParamType = {
  name?: string
  type: string
  value: any
}

export type QuickModeStaceTraceType = {
  functionName: string
  location: LocationWithInstructionIndex | null
}

export const enum DEBUGGER_MODE {
  STATIC = 'static',
  SINGLE = 'single' // single-step
}

export const enum BUILD_STATE {
  RELEASE = 'release',
  DEBUG = 'debug',
  DEBUG_IGNORE_GAS = 'debug_ignore_gas'
}

export type SourceFileType = {
  label: string
  children: {
    label: string
    uri: monaco.Uri
    source?: {
      compilationId: string
      filePath: string
    }
  }[]
}

export type ExtendBreakpoint = Breakpoint & {
  status: BreakpointStatus
  model: monaco.editor.ITextModel
}

type CompilerType = {
  name: string
  version: string
}

export type FetchAndCompileResponse = {
  result: {
    address: string
    compiler: CompilerType
    contracts: any
    id: string
    sources: {
      compiler: CompilerType
      id: string
      language: string
      source: string
      sourcePath: string
    }[]
    unreliableSourceOrder?: boolean
  }[]
  sourceInfo: any
}

export type TxnQueryStateType = Partial<{
  success: boolean
  failed: boolean
  pending: boolean
  type: 'direct' | 'all' | 'internal'
  funsig: string
  contract: Pick<Contract, 'address' | 'chainId' | 'name'>[]
  chain: string[]
}>

export type parseMonacoUriFn = (uri?: monaco.Uri) => { address: string; path: string }

export type PreviewLocation = {
  address: string
  path: string
  startLineNumber: number
  startColumn: number
  endLineNumber: number
  endColumn: number
}

export type FileNameType = {
  address: string
  contractName: string
  path: string
}

export type SymbolType = {
  symbol: string
  file: string
  line: number
  column: number
  contractAddress: string
  kind?: string
}

export type DecodedStructType = {
  pointer: {
    from: {
      offset: number
      slot: string
    }
    to: {
      offset: number
      slot: string
    }
  }
  typeString: string
  value: any
}

export type StorageStateVariableType = {
  decoded: DecodedStructType
  decodedMember?: DecodedStructType
  id: string
  location: {
    compilationId: string
    lines: {
      start: {
        column: number
        line: number
      }
      end: {
        column: number
        line: number
      }
    }
    sourcePath: string
  }
  type: {
    name: string
    type: string
  }
}
