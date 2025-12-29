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

export type CompilerType = {
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
export type GetContractIndexResponse = {
  index?: Index
}
