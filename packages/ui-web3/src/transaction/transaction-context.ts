import { createContext } from 'react'
import { FileNameType, SymbolType } from './types'
import { KeyedMutator } from 'swr'


export const TxIdentifierContext = createContext<'txId.txHash' | 'txId.simulationId' | 'txId.bundleId'>('txId.txHash')
export const SenderContext = createContext<string | undefined>(undefined)
export const ReceiverContext = createContext<string | undefined>(undefined)
export const ChainIdContext = createContext<string | undefined>(undefined)
export const SimulationIdContext = createContext<string | undefined>(undefined)
export const BundleIndexContext = createContext<number | undefined>(undefined)
export const BundleSetIndexContext = createContext<((idx: number) => void) | undefined>(undefined)
export const ChainIdentifierContext = createContext<'chainSpec.chainId' | 'chainSpec.forkId'>('chainSpec.chainId')
export const SimShareIdContext = createContext<string | undefined>(undefined)
export const PriceFetcherContext = createContext(async function priceFetcher(params: any) {
  return {} as any
})
export const OverviewContext = createContext<{
  routeTo: (path?: string, dropBuild?: boolean, newWindow?: boolean) => void
  setMask: (show: boolean) => void
}>({
  routeTo: (path, dropBuild?: boolean) => {
    console.log(path)
  },
  setMask: (show) => {
    console.log(show)
  }
})
export const FileNamesContext = createContext<FileNameType[]>([])
export const SymbolsContext = createContext<SymbolType[]>([])
export const GlobalQueryContext = createContext<Record<string, string>>({})
export const CallTracesContext = createContext<{
  data?: any
  error?: any
  loading?: boolean
  mutate?: KeyedMutator<any>
}>({})
