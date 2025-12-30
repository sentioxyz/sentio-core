import { useCallback, useEffect, Suspense, useState, useRef } from 'react'
import {
  useForm,
  FormProvider,
  useWatch,
  useFormContext
} from 'react-hook-form'
import { ClipLoader } from 'react-spinners'
import Web3 from 'web3'
import { BigDecimal } from '@sentio/bigdecimal'
import { isEqual, merge, pickBy, identity, isEmpty } from 'lodash'
import dayjs from 'dayjs'

import { Button, Input, Switch } from '@sentio/ui-core'
import { DisclosurePanel } from './Panel'
import { FunctionParameter } from './FunctionParameter'
import { FunctionSelect } from './FunctionSelect'
import { OptionalAccessList } from './OptionalAccessList'
import { StateOverride } from './StateOverride'
import { BlockNumberInput } from './BlockNumberInput'
import { TxnNumberInput } from './TxnNumberInput'
import { EncodedCallData } from './EncodedCallData'
import { CallDataSwitch } from './CallDataSwitch'
import BaseFee from './BaseFee'
import {
  AmountUnitSelect,
  genCoefficient,
  getWeiAmount
} from './AmountUnitSelect'
import {
  useSimulatorContext,
  SimulationFormState,
  ContractSelectType
} from './SimulatorContext'
import {
  SimulationFormType,
  Contract as ContractType,
  Simulation,
  SimulateTransactionRequest,
  SimulateTransactionResponse,
  AmountUnit
} from './types'
import { ContractName } from './ContractName'

const BD = BigDecimal.clone({
  EXPONENTIAL_AT: [-30, 30]
})

function parseTime(input: string | number | undefined) {
  if (!input) return undefined
  if (typeof input === 'number') return dayjs(input)
  if (/^\d+$/.test(input)) {
    if (input.length === 13) return dayjs(Number(input))
    else if (input.length === 10) return dayjs.unix(Number(input))
  }
  return dayjs(input)
}

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

function decimalToHex(input: number) {
  if (input === 0) return '0'
  return '0x' + input.toString(16)
}

const getHexStringByMultiple = (value: any, power: number) => {
  try {
    const bd = BD(value).multipliedBy(BD(10).pow(power))
    return '0x' + bd.toString(16)
  } catch {
    return '0x0'
  }
}

function getDefaultValue(
  defaultValue?: Partial<SimulationFormType>,
  atomFormState?: SimulationFormState
) {
  return merge(
    {
      blockNumber: 0,
      txIndex: 0,
      from: '0x',
      gas: 8000000,
      gasPrice: 0,
      value: 0,
      header: {
        blockNumber: undefined,
        blockNumberState: false,
        timestamp: undefined,
        timestampState: false
      },
      stateOverride: [],
      function: null,
      input: '',
      contract: null,
      accessList: []
    },
    defaultValue
  )
}

function FormValueWatcher({
  onChange
}: {
  onChange: SimulationProps['onChange']
}) {
  const { watch } = useFormContext()
  const { simulationFormState: atomFormState } = useSimulatorContext()
  const formValues = watch()

  const atomFormStateRef = useRef<any>(null)
  const formValuesRef = useRef<any>(null)

  useEffect(() => {
    if (
      isEqual(atomFormStateRef.current, atomFormState) &&
      isEqual(formValuesRef.current, formValues)
    ) {
      return
    }
    onChange?.(formValues, atomFormState)
    atomFormStateRef.current = atomFormState
    formValuesRef.current = formValues
  }, [formValues, atomFormState, onChange])

  return null
}

export const multiplyAmount = {
  [AmountUnit.Wei]: 0,
  [AmountUnit.Gwei]: 9,
  [AmountUnit.Ether]: 18
}

export function genDataFrom(
  values: SimulationFormType,
  atomFormState?: SimulationFormState,
  chainIdentifier = 'chainSpec.chainId'
) {
  if (!values.contract) return

  const web3 = new Web3()
  const valueUnit = (atomFormState?.valueUnit ?? AmountUnit.Ether) as AmountUnit
  const gasPriceUnit = (atomFormState?.gasPriceUnit ??
    AmountUnit.Gwei) as AmountUnit

  const simulation: Simulation = {
    chainSpec: {
      [chainIdentifier === 'chainSpec.chainId' ? 'chainId' : 'forkId']:
        values.contract?.chainId ?? '1'
    },
    blockNumber: values.blockNumber.toString(),
    transactionIndex: atomFormState?.usePendingBlock
      ? '0'
      : values.txIndex.toString(),
    from: values.from,
    to: values.contract?.address,
    value: getHexStringByMultiple(
      values.value,
      multiplyAmount[valueUnit] ?? 18
    ) as string,
    gas: getHexStringByMultiple(values.gas, 0) as string,
    gasPrice: getHexStringByMultiple(
      values.gasPrice.toString(),
      multiplyAmount[gasPriceUnit] ?? 9
    ) as string,
    blockOverride: {},
    accessList: values.accessList
  }

  if (atomFormState?.useRawInput) {
    if (values.input) {
      simulation.input = values.input
    }
  } else if (values.function) {
    simulation.input = web3.eth.abi.encodeFunctionCall(
      values.function as any,
      values.functionParams
        ? values.functionParams.map((item) => item.value)
        : []
    )
  }

  if (values.stateOverride) {
    const res: Simulation['stateOverrides'] = {}
    values.stateOverride.forEach((item) => {
      const { contract, balance, storage } = item
      if (!balance && !storage) return

      res[contract] = {}
      if (balance) {
        res[contract] = {
          ...res[contract],
          balance: '0x' + BigDecimal(balance).toString(16)
        }
      }
      if (storage && storage.length > 0) {
        res[item.contract].state = storage?.reduce(
          (acc, cur) => {
            acc[cur.key] = cur.value
            return acc
          },
          {} as Record<string, string>
        )
      }
    })
    if (!isEmpty(res)) {
      simulation.stateOverrides = res
    }
  }

  if (values.header.blockNumber) {
    simulation.blockOverride!.blockNumber = decimalToHex(
      values.header.blockNumber
    )
  }
  if (values.header.timestamp) {
    const time = parseTime(values.header.timestamp)
    if (time && time.isValid()) {
      simulation.blockOverride!.timestamp = decimalToHex(time.unix())
    }
  }
  return simulation
}

const contractAddressRegex = /^0x[a-fA-F0-9]{40}$/

export const NewSimulation = ({
  projectId,
  onClose,
  defaultValue,
  onSuccess,
  hideSourceOverride,
  onProjectChange,
  onRequestAPI,
  relatedContracts,
  onChange,
  originTxHash,
  hideProjectSelect,
  hideNetworkSelect,
  hideContractName
}: SimulationProps) => {
  const {
    simulationFormState: atomFormState,
    setSimulationFormState: setFormState
  } = useSimulatorContext()
  const atomFormStateRef = useRef(atomFormState)
  atomFormStateRef.current = atomFormState

  const [apiError, setAPIError] = useState<string>('')
  const mergedDefaultValue = getDefaultValue(defaultValue, atomFormState)

  const form = useForm<SimulationFormType>({
    defaultValues: mergedDefaultValue,
    mode: onChange ? 'onBlur' : 'onSubmit'
  })

  const {
    register,
    watch,
    setValue,
    control,
    trigger,
    handleSubmit,
    formState,
    getValues
  } = form
  const { isSubmitting, errors } = formState

  const onSubmit = useCallback(
    handleSubmit(async (values) => {
      const atomFormState = atomFormStateRef.current
      try {
        setAPIError('')
        const simulation = genDataFrom(values, atomFormState)
        if (!simulation) return

        if (originTxHash) {
          simulation.originTxHash = originTxHash
        }

        const req: SimulateTransactionRequest = pickBy(
          {
            projectOwner: values.projectOwner,
            projectSlug: values.projectSlug,
            simulation
          },
          identity
        )

        const res = onRequestAPI
          ? await onRequestAPI(req)
          : { simulation: { id: 'mock' } }

        if (res.simulation) {
          onClose?.()
          onSuccess?.({
            ...res,
            projectOwner: req.projectOwner,
            projectSlug: req.projectSlug
          })
        }
      } catch (e: any) {
        if (e?.body?.message) {
          setAPIError(e.body.message as string)
        } else {
          setAPIError('Simulation failed, please try again later.')
        }
      }
    }),
    [handleSubmit, onClose, onSuccess, onRequestAPI, originTxHash]
  )

  const setPendingBlock = () => {
    setFormState({
      usePendingBlock: !atomFormState.usePendingBlock
    })
  }

  const contract = watch('contract')
  const func = watch('function')
  const chainId = watch('contract.chainId')

  useEffect(() => {
    const subscription = watch((value, { name, type }) => {
      if (name === 'header.blockNumberState') {
        if (value.header && value.header?.blockNumberState === false) {
          setValue('header.blockNumber', undefined)
        }
      } else if (name === 'header.timestampState') {
        if (value.header && value.header?.timestampState === false) {
          setValue('header.timestamp', undefined)
        }
      }
    })
    return () => subscription.unsubscribe()
  }, [watch, setValue])

  useEffect(() => {
    if (defaultValue?.input) {
      setFormState({
        useRawInput: true
      })
    }
  }, [defaultValue?.input])

  useEffect(() => {
    form.reset(getDefaultValue(defaultValue, atomFormStateRef.current))
  }, [defaultValue?.sourceOverrides])

  const stateOverrideData = useWatch({
    control,
    name: 'stateOverride'
  })

  const accessListData = useWatch({
    control,
    name: 'accessList'
  })

  const overrideTimestamp = watch('header.timestamp')

  return (
    <FormProvider {...form}>
      <div className="relative grid h-full w-full grid-cols-1">
        <div className="px-4 pb-4 pt-2">
          <div>
            <div className="mb-4 space-y-2">
              <div className="text-text-foreground text-base font-bold">
                Sender
              </div>
              <Input
                error={errors.from}
                {...register('from', {
                  required: true,
                  minLength: {
                    value: 42,
                    message: 'Contract Address should contain 42 characters.'
                  },
                  maxLength: {
                    value: 42,
                    message:
                      'Contract Address should not contain more than 42 characters.'
                  },
                  pattern: {
                    value: contractAddressRegex,
                    message:
                      'Contract address is not valid, please check again.'
                  }
                })}
              />
            </div>

            <div className="text-text-foreground text-base font-bold">
              Receiver
            </div>
            <div className="mt-2 space-y-2">
              <Input
                error={errors.contract as any}
                placeholder="Contract address (0x...)"
                {...register('contract.address', { required: true })}
              />
              <ContractName address={contract?.address} />
            </div>

            <div className="my-4">
              <div className="flex w-full justify-between">
                <div className="text-text-foreground text-base font-bold">
                  Call Data
                </div>
              </div>
              <div className="mt-2 flex items-center">
                <CallDataSwitch
                  inputType={atomFormState.useRawInput}
                  onChange={(val) => {
                    setFormState({ useRawInput: val })
                  }}
                />
              </div>
              <EncodedCallData
                control={control}
                className={atomFormState.useRawInput ? '' : 'hidden'}
              />
              <div
                className={`mt-2 space-y-3 rounded bg-gray-50 p-2 ${atomFormState.useRawInput ? 'hidden' : ''}`}
              >
                <div className="text-ilabel text-text-foreground font-bold">
                  Function
                </div>
                <Suspense
                  fallback={
                    <div className="flex h-16 w-full animate-pulse items-center justify-center gap-2 rounded-md bg-gray-200">
                      <ClipLoader
                        loading
                        color="#3B82F6"
                        size={24}
                        cssOverride={{ borderWidth: 3 }}
                      />
                      <span className="text-gray text-icontent">
                        Fetching ABI
                      </span>
                    </div>
                  }
                >
                  <FunctionSelect control={control} />
                </Suspense>
                {contract && func && (
                  <FunctionParameter
                    control={control}
                    copyBtnClassName="absolute right-2 top-6"
                    lineClassName="pr-12 relative"
                    copyAllClassName="pl-0 text-ilabel"
                  />
                )}
              </div>
            </div>
          </div>

          <div className="text-icontent relative -mx-2 space-y-4 font-medium">
            <DisclosurePanel
              defaultOpen={true}
              title={
                <div className="text-text-foreground text-base font-bold">
                  Transaction Parameters
                </div>
              }
              titleClassName="px-2 rounded-md"
              className="px-2"
            >
              <div className="mt-2 space-y-4">
                <div className="flex items-center gap-1.5">
                  <Switch
                    checked={atomFormState.usePendingBlock}
                    onChange={setPendingBlock}
                    size="sm"
                  />
                  <span className="text-xs font-medium">Use Pending Block</span>
                </div>
                <BlockNumberInput />
                <TxnNumberInput />
                <div className="space-y-2">
                  <div className="text-ilabel text-text-foreground font-medium">
                    Gas Limit
                  </div>
                  <div>
                    <input
                      {...register('gas', { valueAsNumber: true })}
                      className="border-border-color w-full rounded-md border p-2 font-normal"
                    />
                  </div>
                </div>
                <div className="space-y-2">
                  <div className="text-ilabel text-text-foreground font-medium">
                    Gas Price
                  </div>
                  <div className="relative">
                    <input
                      {...register('gasPrice')}
                      className="border-border-color w-full rounded-md border p-2 pr-14 font-normal"
                    />
                    <div className="absolute right-0.5 top-0.5 inline-flex w-20 items-center">
                      <AmountUnitSelect
                        value={atomFormState.gasPriceUnit}
                        onChange={(value) => {
                          const prevGasPriceUnit = atomFormState.gasPriceUnit
                          const prevGasPrice = getValues('gasPrice')
                          setFormState({ gasPriceUnit: value })
                          if (prevGasPriceUnit !== value) {
                            const newGasPrice = BD(prevGasPrice)
                            setValue(
                              'gasPrice',
                              newGasPrice
                                .multipliedBy(
                                  genCoefficient(prevGasPriceUnit, value)
                                )
                                .toString()
                            )
                          }
                        }}
                        buttonClassName="border-none !py-1.5"
                        className="w-full"
                      />
                    </div>
                  </div>
                </div>
                <div className="space-y-2">
                  <div className="text-ilabel text-text-foreground font-medium">
                    Value
                  </div>
                  <div className="relative">
                    <input
                      {...register('value')}
                      className="border-border-color w-full rounded-md border p-2 pr-14 font-normal"
                    />
                    <div className="absolute right-0.5 top-0.5 inline-flex w-20 items-center">
                      <AmountUnitSelect
                        value={atomFormState.valueUnit}
                        onChange={(value) => {
                          const prevValueUnit = atomFormState.valueUnit
                          const prevValue = getValues('value')
                          setFormState({ valueUnit: value })
                          if (prevValueUnit !== value) {
                            const newValue = BD(prevValue)
                            setValue(
                              'value',
                              newValue
                                .multipliedBy(
                                  genCoefficient(prevValueUnit, value)
                                )
                                .toString()
                            )
                          }
                        }}
                        buttonClassName="border-none !py-1.5"
                        className="w-full"
                      />
                    </div>
                  </div>
                </div>
                <BaseFee />
              </div>
            </DisclosurePanel>

            <DisclosurePanel
              titleClassName="px-2 rounded-md"
              className="px-2"
              title={
                <div className="text-text-foreground text-base font-bold">
                  Block Header Overrides
                </div>
              }
              defaultOpen={
                mergedDefaultValue.header?.blockNumberState ||
                mergedDefaultValue.header?.timestampState
              }
            >
              <div className="mt-2 space-y-4">
                <div className="space-y-2">
                  <div className="flex w-full justify-between">
                    <div className="text-ilabel text-text-foreground font-medium">
                      Block Number
                    </div>
                    <div className="flex items-center gap-1.5">
                      <span className="text-xs font-medium">
                        Override Block Number
                      </span>
                      <Switch
                        checked={watch('header.blockNumberState')}
                        onChange={(checked) => {
                          setValue('header.blockNumberState', checked)
                          if (!checked) {
                            setValue('header.blockNumber', undefined)
                          }
                        }}
                        size="sm"
                      />
                    </div>
                  </div>
                  <div>
                    <input
                      placeholder="/"
                      {...register('header.blockNumber', {
                        valueAsNumber: true,
                        disabled: watch('header.blockNumberState') === false
                      })}
                      className="border-border-color w-full rounded-md border p-2 font-normal"
                    />
                  </div>
                </div>
                <div className="space-y-2">
                  <div className="flex w-full justify-between">
                    <div className="text-ilabel text-text-foreground font-medium">
                      Timestamp
                    </div>
                    <div className="flex items-center gap-1.5">
                      <span className="text-xs font-medium">
                        Override Timestamp
                      </span>
                      <Switch
                        checked={watch('header.timestampState')}
                        onChange={(checked) => {
                          setValue('header.timestampState', checked)
                          if (!checked) {
                            setValue('header.timestamp', undefined)
                          }
                        }}
                        size="sm"
                      />
                    </div>
                  </div>
                  <div className="relative">
                    <Input
                      error={errors.header?.timestamp}
                      placeholder={
                        watch('header.timestampState') === false
                          ? '/'
                          : 'Date or milliseconds timestamp'
                      }
                      disabled={watch('header.timestampState') === false}
                      {...register('header.timestamp', {
                        validate: (value: any) => {
                          if (value === undefined || value === '') return true
                          const time = parseTime(value)
                          if (time && time.isValid()) return true
                          return 'Invalid timestamp'
                        }
                      })}
                    />
                    <span className="absolute right-2 top-2 text-xs text-gray-500">
                      {overrideTimestamp
                        ? parseTime(overrideTimestamp)?.format(
                            'YYYY-MM-DD HH:mm:ss'
                          ) || ''
                        : ''}
                    </span>
                  </div>
                </div>
              </div>
            </DisclosurePanel>

            <DisclosurePanel
              titleClassName="px-2 rounded-md"
              className="px-2"
              title={
                <div className="text-text-foreground text-base font-bold">
                  State Overrides
                </div>
              }
              defaultOpen={stateOverrideData && stateOverrideData.length > 0}
            >
              <StateOverride
                control={control}
                relatedContracts={relatedContracts}
              />
            </DisclosurePanel>

            <DisclosurePanel
              titleClassName="px-2 rounded-md"
              className="px-2"
              title={
                <div className="text-text-foreground text-base font-bold">
                  Optional Access Lists
                </div>
              }
              defaultOpen={accessListData && accessListData.length > 0}
            >
              <div className="space-y-3 pt-2">
                <div className="text-xs font-medium text-gray-800">
                  This Simulation will have the{' '}
                  <a
                    href="https://blog.ethereum.org/2021/03/08/ethereum-berlin-upgrade-announcement/"
                    target="_blank"
                    rel="noreferrer"
                    className="underline"
                  >
                    Berlin fork
                  </a>{' '}
                  enabled, which includes support for{' '}
                  <a
                    href="https://eips.ethereum.org/EIPS/eip-2930"
                    target="_blank"
                    rel="noreferrer"
                    className="underline"
                  >
                    EIP-2930 Optional access lists
                  </a>
                  .
                </div>
                <OptionalAccessList />
              </div>
            </DisclosurePanel>
          </div>
        </div>

        <div className="sticky bottom-0 z-[1] flex w-full items-center justify-between bg-gray-50/50 px-2 py-2">
          {apiError ? (
            <span className="text-sm text-red-600">{apiError}</span>
          ) : (
            <span></span>
          )}
          {onChange ? null : (
            <div className="flex items-center gap-2">
              <Button
                role="primary"
                size="md"
                onClick={onSubmit}
                processing={isSubmitting}
              >
                Simulate Transaction
              </Button>
            </div>
          )}
        </div>
      </div>
      <FormValueWatcher onChange={onChange} />
    </FormProvider>
  )
}
