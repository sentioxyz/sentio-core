import React, {
  createContext,
  useContext,
  useState,
  useCallback,
  ReactNode
} from 'react'
import { AmountUnit } from './types'

export enum ContractSelectType {
  Project = 1,
  Inline = 2,
  Compilation = 3
}

// simulation form state
export type SimulationFormState = {
  useCustomContract: ContractSelectType
  usePendingBlock: boolean
  useCustomBlockNumber: boolean
  useCustomTimestamp: boolean
  useCustomFromAddress: boolean
  useCustomGas: boolean
  useRawInput: boolean
  gasPriceUnit: AmountUnit
  valueUnit: AmountUnit
}

interface SimulatorContextType {
  // Form state
  simulationFormState: SimulationFormState
  setSimulationFormState: (state: Partial<SimulationFormState>) => void

  // Basic atoms
  contractAddress: string
  setContractAddress: (address: string) => void

  contractNetwork: string
  setContractNetwork: (network: string) => void

  chainIdentifier: 'chainSpec.chainId' | 'chainSpec.forkId'
  setChainIdentifier: (
    identifier: 'chainSpec.chainId' | 'chainSpec.forkId'
  ) => void

  useCompilation: boolean
  setUseCompilation: (value: boolean) => void

  compilationId: string | undefined
  setCompilationId: (id: string | undefined) => void

  blockNumber: number
  setBlockNumber: (number: number) => void

  simId: string | undefined
  setSimId: (id: string | undefined) => void

  simBundleId: string | undefined
  setSimBundleId: (id: string | undefined) => void

  // Derived atoms
  contractName: string
  setContractName: (name: string) => void

  contractFunctions: {
    wfunctions?: any[]
    rfunctions?: any[]
  }
  setContractFunctions: (functions: {
    wfunctions?: any[]
    rfunctions?: any[]
  }) => void

  latestBlockNumber: { blockNumber?: string }
  setLatestBlockNumber: (block: { blockNumber?: string }) => void

  blockSummary: { transactionCount?: number }
  setBlockSummary: (summary: { transactionCount?: number }) => void
}

const SimulatorContext = createContext<SimulatorContextType | undefined>(
  undefined
)

export function SimulatorProvider({ children }: { children: ReactNode }) {
  const [simulationFormState, _setSimulationFormState] =
    useState<SimulationFormState>({
      useCustomContract: ContractSelectType.Project,
      usePendingBlock: false,
      useCustomBlockNumber: false,
      useCustomTimestamp: false,
      useCustomFromAddress: true,
      useCustomGas: true,
      useRawInput: false,
      gasPriceUnit: AmountUnit.Gwei,
      valueUnit: AmountUnit.Ether
    })

  const setSimulationFormState = useCallback(
    (newState: Partial<SimulationFormState>) => {
      _setSimulationFormState((prev) => {
        // Apply the same reducer logic as before
        if (prev.usePendingBlock) {
          if (newState.useCustomBlockNumber || newState.useCustomTimestamp) {
            return prev
          }
        }
        return { ...prev, ...newState }
      })
    },
    []
  )

  const [contractAddress, setContractAddress] = useState<string>('')
  const [contractNetwork, setContractNetwork] = useState<string>('1')
  const [chainIdentifier, setChainIdentifier] = useState<
    'chainSpec.chainId' | 'chainSpec.forkId'
  >('chainSpec.chainId')
  const [useCompilation, setUseCompilation] = useState<boolean>(false)
  const [compilationId, setCompilationId] = useState<string | undefined>(
    undefined
  )
  const [blockNumber, setBlockNumber] = useState<number>(0)
  const [simId, setSimId] = useState<string | undefined>(undefined)
  const [simBundleId, setSimBundleId] = useState<string | undefined>(undefined)

  // Derived state
  const [contractName, setContractName] = useState<string>('')
  const [contractFunctions, setContractFunctions] = useState<{
    wfunctions?: any[]
    rfunctions?: any[]
    name?: string
  }>({})
  const [latestBlockNumber, setLatestBlockNumber] = useState<{
    blockNumber?: string
  }>({})
  const [blockSummary, setBlockSummary] = useState<{
    transactionCount?: number
  }>({})

  const value: SimulatorContextType = {
    simulationFormState,
    setSimulationFormState,
    contractAddress,
    setContractAddress,
    contractNetwork,
    setContractNetwork,
    chainIdentifier,
    setChainIdentifier,
    useCompilation,
    setUseCompilation,
    compilationId,
    setCompilationId,
    blockNumber,
    setBlockNumber,
    simId,
    setSimId,
    simBundleId,
    setSimBundleId,
    contractName,
    setContractName,
    contractFunctions,
    setContractFunctions,
    latestBlockNumber,
    setLatestBlockNumber,
    blockSummary,
    setBlockSummary
  }

  return (
    <SimulatorContext.Provider value={value}>
      {children}
    </SimulatorContext.Provider>
  )
}

export function useSimulatorContext() {
  const context = useContext(SimulatorContext)
  if (context === undefined) {
    throw new Error(
      'useSimulatorContext must be used within a SimulatorProvider'
    )
  }
  return context
}
