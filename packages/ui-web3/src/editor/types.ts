import * as monaco from 'monaco-editor'
import { Index } from '@sentio/scip'

export type parseMonacoUriFn = (uri?: monaco.Uri) => {
  address: string
  path: string
}

export type PreviewLocation = {
  address: string
  path: string
  startLineNumber: number
  startColumn: number
  endLineNumber: number
  endColumn: number
}

export enum CompilerType {
  UNKNOWN = 0,
  SOLIDITY = 1,
  VYPER = 2,
  MOVE = 3
}

export type FetchAndCompileResponse = {
  result: {
    address: string
    compiler: CompilerType
    contracts: any
    id: string
    networkId: string
    sources: any
  }
}

export type GetContractIndexResponse = {
  index?: Index
}
