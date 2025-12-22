import { useCallback, useEffect, useState } from 'react'
import { useForm, FormProvider, useWatch } from 'react-hook-form'
import Web3 from 'web3'
import { BigDecimal } from '@sentio/bigdecimal'
import { FunctionParameter } from './FunctionParameter'
import { FunctionSelect } from './FunctionSelect'
import { DisclosurePanel } from './Panel'
import { AmountUnitSelect, getWeiAmount } from './AmountUnitSelect'
import type {
  SimulationFormType,
  Contract,
  AbiFunction,
  FunctionParam,
  AccessListItem,
  StateOverrideItem,
  SourceOverrideItem,
  Simulation,
  SimulateTransactionRequest,
  SimulateTransactionResponse,
  AmountUnit
} from './types'

export interface SimulationProps {
  projectId?: string
  onClose?: () => void
  defaultValue?: Partial<SimulationFormType>
  relatedContracts?: {
    address: string
    name: string
  }[]
  onSuccess?: (
    data: SimulateTransactionResponse & {
      projectOwner?: string
      projectSlug?: string
    }
  ) => void
  hideSourceOverride?: boolean
  onProjectChange?: (project: any) => void
  onRequestAPI?: (
    data: SimulateTransactionRequest
  ) => Promise<SimulateTransactionResponse>
  onChange?: (data: Partial<SimulationFormType>, atomState?: any) => void
  originTxHash?: string
  hideProjectSelect?: boolean
  hideNetworkSelect?: boolean
  hideContractName?: boolean
}

const web3 = new Web3()

function decimalToHex(input: number) {
  if (input === 0) {
    return '0x0'
  }
  return '0x' + input.toString(16)
}

export const NewSimulation = ({
  defaultValue,
  onSuccess,
  onRequestAPI,
  onChange,
  onClose,
  hideProjectSelect,
  hideNetworkSelect
}: SimulationProps) => {
  const [loading, setLoading] = useState(false)
  const [useRawInput, setUseRawInput] = useState(false)
  const [valueUnit, setValueUnit] = useState<AmountUnit>('ether' as AmountUnit)
  const [gasPriceUnit, setGasPriceUnit] = useState<AmountUnit>(
    'gwei' as AmountUnit
  )

  const methods = useForm<SimulationFormType>({
    defaultValues: {
      blockNumber: 18500000,
      txIndex: 0,
      from: '',
      to: '',
      gas: 300000,
      gasPrice: 20,
      value: 0,
      input: '',
      header: {
        blockNumberState: false,
        timestampState: false
      },
      ...defaultValue
    }
  })

  const {
    register,
    handleSubmit,
    control,
    setValue,
    watch,
    formState: { errors }
  } = methods
  const contractValue = watch('contract')
  const functionValue = watch('function')

  // Notify onChange
  useEffect(() => {
    if (onChange) {
      const subscription = watch((data) => {
        onChange(data as Partial<SimulationFormType>)
      })
      return () => subscription.unsubscribe()
    }
  }, [onChange, watch])

  const onSubmit = useCallback(
    async (data: SimulationFormType) => {
      setLoading(true)
      try {
        // Encode function call if not using raw input
        let callData = data.input || '0x'
        if (!useRawInput && data.function && data.functionParams) {
          try {
            callData = web3.eth.abi.encodeFunctionCall(
              data.function as any,
              data.functionParams.map((p) => p.value)
            )
          } catch (e) {
            console.error('Failed to encode function call:', e)
          }
        }

        // Convert value and gas price to wei
        const valueInWei = getWeiAmount(
          String(data.value || 0),
          valueUnit
        ).toFixed(0)
        const gasPriceInWei = getWeiAmount(
          String(data.gasPrice || 0),
          gasPriceUnit
        ).toFixed(0)

        const simulation: Simulation = {
          chainSpec: {
            chainId: data.contract?.chainId || '1'
          },
          blockNumber: decimalToHex(data.blockNumber),
          transactionIndex: decimalToHex(data.txIndex || 0),
          from: data.from,
          to: data.to || data.contract?.address,
          value: decimalToHex(parseInt(valueInWei)),
          gas: decimalToHex(data.gas),
          gasPrice: decimalToHex(parseInt(gasPriceInWei)),
          input: callData,
          blockOverride:
            data.header?.blockNumberState || data.header?.timestampState
              ? {
                  blockNumber: data.header.blockNumberState
                    ? decimalToHex(data.header.blockNumber!)
                    : undefined,
                  timestamp: data.header.timestampState
                    ? String(data.header.timestamp)
                    : undefined
                }
              : undefined
        }

        const request: SimulateTransactionRequest = {
          projectOwner: data.projectOwner,
          projectSlug: data.projectSlug,
          simulation
        }

        let response: SimulateTransactionResponse
        if (onRequestAPI) {
          response = await onRequestAPI(request)
        } else {
          // Mock response for demo
          response = {
            simulation: {
              id: 'mock-' + Date.now(),
              status: 'pending'
            }
          }
        }

        if (onSuccess) {
          onSuccess({
            ...response,
            projectOwner: data.projectOwner,
            projectSlug: data.projectSlug
          })
        }
      } catch (error) {
        console.error('Simulation error:', error)
      } finally {
        setLoading(false)
      }
    },
    [onSuccess, onRequestAPI, useRawInput, valueUnit, gasPriceUnit]
  )

  return (
    <FormProvider {...methods}>
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-6 p-6">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-bold text-gray-900">New Simulation</h2>
          {onClose && (
            <button
              type="button"
              onClick={onClose}
              className="text-gray-500 hover:text-gray-700"
            >
              ✕
            </button>
          )}
        </div>

        {/* Contract Address */}
        <div className="space-y-2">
          <label className="block text-sm font-medium text-gray-900">
            Contract Address
          </label>
          <input
            {...register('contract.address', {
              required: 'Contract address is required'
            })}
            className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
            placeholder="0x..."
          />
          {errors.contract?.address && (
            <p className="text-sm text-red-600">
              {errors.contract.address.message}
            </p>
          )}
        </div>

        {/* Chain ID */}
        {!hideNetworkSelect && (
          <div className="space-y-2">
            <label className="block text-sm font-medium text-gray-900">
              Chain ID
            </label>
            <input
              {...register('contract.chainId')}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
              placeholder="1"
              defaultValue="1"
            />
          </div>
        )}

        {/* From Address */}
        <div className="space-y-2">
          <label className="block text-sm font-medium text-gray-900">
            From Address
          </label>
          <input
            {...register('from', { required: 'From address is required' })}
            className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
            placeholder="0x..."
          />
          {errors.from && (
            <p className="text-sm text-red-600">{errors.from.message}</p>
          )}
        </div>

        {/* Block Number */}
        <div className="space-y-2">
          <label className="block text-sm font-medium text-gray-900">
            Block Number
          </label>
          <input
            type="number"
            {...register('blockNumber', {
              required: true,
              valueAsNumber: true
            })}
            className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
          />
        </div>

        {/* Input Method Toggle */}
        <div className="space-y-2">
          <div className="flex gap-4">
            <label className="inline-flex items-center">
              <input
                type="radio"
                checked={!useRawInput}
                onChange={() => setUseRawInput(false)}
                className="mr-2"
              />
              <span className="text-sm">Choose function and parameters</span>
            </label>
            <label className="inline-flex items-center">
              <input
                type="radio"
                checked={useRawInput}
                onChange={() => setUseRawInput(true)}
                className="mr-2"
              />
              <span className="text-sm">Enter raw input data</span>
            </label>
          </div>
        </div>

        {/* Function Selection or Raw Input */}
        {useRawInput ? (
          <div className="space-y-2">
            <label className="block text-sm font-medium text-gray-900">
              Call Data
            </label>
            <input
              {...register('input')}
              className="w-full rounded-md border border-gray-300 px-3 py-2 font-mono text-sm"
              placeholder="0x..."
            />
          </div>
        ) : (
          <>
            <FunctionSelect control={control} functions={contractValue?.abi} />
            {functionValue && <FunctionParameter control={control} />}
          </>
        )}

        {/* Value */}
        <div className="space-y-2">
          <label className="block text-sm font-medium text-gray-900">
            Value
          </label>
          <div className="flex gap-2">
            <input
              type="number"
              {...register('value')}
              className="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm"
              placeholder="0"
            />
            <AmountUnitSelect value={valueUnit} onChange={setValueUnit} />
          </div>
        </div>

        {/* Gas Limit */}
        <div className="space-y-2">
          <label className="block text-sm font-medium text-gray-900">
            Gas Limit
          </label>
          <input
            type="number"
            {...register('gas', { valueAsNumber: true })}
            className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
          />
        </div>

        {/* Gas Price */}
        <div className="space-y-2">
          <label className="block text-sm font-medium text-gray-900">
            Gas Price
          </label>
          <div className="flex gap-2">
            <input
              type="number"
              {...register('gasPrice')}
              className="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm"
            />
            <AmountUnitSelect value={gasPriceUnit} onChange={setGasPriceUnit} />
          </div>
        </div>

        {/* Advanced Options */}
        <DisclosurePanel title="Advanced Options" defaultOpen={false}>
          <div className="space-y-4 pt-4">
            {/* Block Override */}
            <div className="space-y-2">
              <label className="inline-flex items-center">
                <input
                  type="checkbox"
                  {...register('header.blockNumberState')}
                  className="mr-2 rounded"
                />
                <span className="text-sm">Override Block Number</span>
              </label>
              <input
                type="number"
                {...register('header.blockNumber', { valueAsNumber: true })}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                disabled={!watch('header.blockNumberState')}
              />
            </div>

            {/* Timestamp Override */}
            <div className="space-y-2">
              <label className="inline-flex items-center">
                <input
                  type="checkbox"
                  {...register('header.timestampState')}
                  className="mr-2 rounded"
                />
                <span className="text-sm">Override Timestamp</span>
              </label>
              <input
                type="number"
                {...register('header.timestamp')}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                disabled={!watch('header.timestampState')}
              />
            </div>

            {/* Transaction Index */}
            <div className="space-y-2">
              <label className="block text-sm font-medium text-gray-900">
                Transaction Index
              </label>
              <input
                type="number"
                {...register('txIndex', { valueAsNumber: true })}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
              />
            </div>
          </div>
        </DisclosurePanel>

        {/* Submit Button */}
        <div className="flex gap-3">
          <button
            type="submit"
            disabled={loading}
            className="flex-1 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:bg-gray-400"
          >
            {loading ? 'Simulating...' : 'Simulate Transaction'}
          </button>
          {onClose && (
            <button
              type="button"
              onClick={onClose}
              className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              Cancel
            </button>
          )}
        </div>
      </form>
    </FormProvider>
  )
}

// Re-export types for use in other components
export type {
  SimulationFormType,
  Contract,
  AbiFunction,
  FunctionParam,
  AccessListItem,
  StateOverrideItem,
  SourceOverrideItem,
  Simulation,
  SimulateTransactionRequest,
  SimulateTransactionResponse,
  AmountUnit
}
