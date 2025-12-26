import { OneOf } from './base'
import { Transaction } from './simulation'

type BaseEvmSearchTransactionsRequest = {
  chainId?: string[]
  address?: string[]
  includeDirect?: boolean
  includeTrace?: boolean
  includeIn?: boolean
  includeOut?: boolean
  transactionStatus?: number[]
  limit?: number
  pageToken?: Uint8Array
}

export type EvmSearchTransactionsRequest = BaseEvmSearchTransactionsRequest
  & OneOf<{ startBlock: string }>
  & OneOf<{ endBlock: string }>
  & OneOf<{ startTimestamp: string }>
  & OneOf<{ endTimestamp: string }>
  & OneOf<{ methodSignature: string }>


type BaseEvmRawTransaction = {
  hash?: string
  blockNumber?: string
  isIn?: boolean
  trace?: boolean
  tx?: Transaction
  json?: string
  timestamp?: string
  transactionStatus?: number
  methodSignature?: string
}

export type EvmRawTransaction = BaseEvmRawTransaction
  & OneOf<{ methodSignatureText: string }>
  & OneOf<{ abiItem: string }>

export type EvmSearchTransactionsResponse = {
  transactions?: EvmRawTransaction[]
  nextPageToken?: Uint8Array
}