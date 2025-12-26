import { atom } from 'jotai'
import { atomWithReducer } from 'jotai/utils'
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

const stateReducer = (
  prev: SimulationFormState,
  action: Partial<SimulationFormState>
) => {
  if (prev.usePendingBlock) {
    if (action.useCustomBlockNumber || action.useCustomTimestamp) {
      return prev
    }
  }

  return { ...prev, ...action }
}

export const simulationFormState = atomWithReducer<
  SimulationFormState,
  Partial<SimulationFormState>
>(
  {
    useCustomContract: ContractSelectType.Project,
    usePendingBlock: false,
    useCustomBlockNumber: false,
    useCustomTimestamp: false,
    useCustomFromAddress: true,
    useCustomGas: true,
    useRawInput: false,
    gasPriceUnit: AmountUnit.Gwei,
    valueUnit: AmountUnit.Ether
  },
  stateReducer
)

export const contractAddress = atom<string>('')
export const contractNetwork = atom<string>('1')
export const chainIdentifier = atom<'chainSpec.chainId' | 'chainSpec.forkId'>(
  'chainSpec.chainId'
)
export const useCompilation = atom<boolean>(false)
export const compilationId = atom<string | undefined>(undefined)
export const blockNumber = atom<number>(0)
export const simId = atom<string | undefined>(undefined)
export const simBundleId = atom<string | undefined>(undefined)

// Derived atoms
export const contractName = atom<string>('')
export const contractFunctions = atom<{
  wfunctions?: any[]
  rfunctions?: any[]
  name?: string
}>({})
export const latestBlockNumber = atom<{ blockNumber?: string }>({})
export const blockSummary = atom<{ transactionCount?: number }>({})
