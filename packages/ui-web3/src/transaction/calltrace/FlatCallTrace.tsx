import {
  LocationWithInstructionIndex,
  DecodedCallTrace
} from '@sentio/debugger-common'
import React, {
  useCallback,
  useRef,
  useContext,
  useEffect,
  useMemo,
  memo,
  createContext,
  useState,
  CSSProperties
} from 'react'
import {
  ArrowRightIcon,
  ExclamationTriangleIcon
} from '@heroicons/react/24/outline'
import {
  PopoverTooltip,
  CopyButton,
  DisclosurePanel,
  FlatTree,
  DataNode,
  ROOT_KEY
} from '@sentio/ui-core'
import { cx } from 'class-variance-authority'
import { isString, isArray, sortBy, upperFirst } from 'lodash'
import {
  getNumberWithDecimal,
  isZeroValue,
  parseCompilationId,
  parseHex,
  setCallTraceKeys,
  filterFundTraces
} from '../helpers'
import { ContractRightTriangleIcon, ContractDebugIcon } from '../Icons'
import {
  ExtendedLog,
  ExtendedCall,
  Transaction,
  ExtendedStorage
} from '../types'
import {
  ContractParam,
  ContractAddress,
  RawParam,
  CopyableParam
} from '../ContractComponents'
import { cva } from 'class-variance-authority'
import {
  ChainIdContext,
  OverviewContext,
  GlobalQueryContext
} from '../transaction-context'
import { getNativeToken } from '../ERC20Token'
import { FloatingDelayGroup } from '@floating-ui/react'
import { LuRoute } from 'react-icons/lu'
import { SubFundflowProvider, useSubFundflow } from './SubFundflow'
import { DecodedVariable } from './DecodedVariable'

const CallTraceContext = createContext<{
  showGas?: boolean
}>({})

// Contexts that may not exist in ui-web3
const SvgFolderContext = createContext<string>('')

// Helper function to check if there is sub fund flow data for a given call trace
const hasSubFundFlow = (data: ExtendedCall, chainId?: string): boolean => {
  if (!data || !chainId) return false

  try {
    const fundItems = filterFundTraces(data, chainId)
    return fundItems.length > 0
  } catch {
    return false
  }
}

const opLabelClass = cva(
  'mr-2 flex items-center gap-2 rounded border px-2 py-0.5 font-normal',
  {
    variants: {
      status: {
        normal: 'text-primary-700/90 border-primary-300/90 bg-primary-50',
        external:
          'text-deep-purple/90 border-deep-purple-300/90 bg-deep-purple-50 dark:text-deep-purple-700/90 dark:border-deep-purple-700/30 dark:bg-deep-purple-500/10',
        danger: 'text-red/90 border-red-300/90 bg-red-50',
        event: 'text-cyan/90 border-cyan-300/90 bg-cyan-50',
        invalid: 'text-gray-500 border-gray-300  bg-gray-50',
        storage: 'text-orange-800/90 border-orange-300/90 bg-orange-50'
      }
    },
    defaultVariants: {
      status: 'normal'
    }
  }
)

function displayBigInt(value?: string) {
  const bigInt = parseHex(value || '0')
  return bigInt.toLocaleString()
}

interface LogEventNodeProps {
  data: ExtendedLog
  depth?: number
  editorNode?: React.ReactNode
  expander?: number
  currentKey?: string
  supportToDebug?: boolean
}

const LogEventNode = memo(function LogEventNode({
  data,
  currentKey,
  supportToDebug,
  ...params
}: LogEventNodeProps) {
  const {
    address,
    events,
    location,
    name,
    data: rawData,
    wkey,
    error,
    parentError,
    topics
  } = data
  const { instructionIndex: index } = location
  const isSelected = currentKey === wkey
  const domRef = useRef<HTMLDivElement>(null)
  const { routeTo, setMask } = useContext(OverviewContext) || {}
  const toDebug = useCallback(async () => {
    setMask?.(true)
    try {
      setMask?.(false)
      routeTo?.(`debug?trace=${index}`, true)
    } catch {
      setMask?.(false)
    }
  }, [setMask, index, routeTo])
  let depth = 0
  if (wkey) {
    depth = wkey?.split('.')?.length ? wkey.split('.').length - 1 : 0
  }

  return (
    <div className="relative font-mono">
      <div className="flex">
        <div
          className={opLabelClass({
            status: error ? 'danger' : parentError ? 'invalid' : 'event'
          })}
        >
          {error ? (
            <span className="inline-block align-text-bottom">
              <PopoverTooltip
                hideArrow
                strategy="fixed"
                text={<span className="text-red">{error}</span>}
              >
                <ExclamationTriangleIcon className="text-red h-4 w-4" />
              </PopoverTooltip>
            </span>
          ) : null}
          <span>{depth}</span>
          <ArrowRightIcon className="h-4 w-4" />
          <span>EVENT</span>
        </div>
        <div className="inline-flex w-full items-center gap-4">
          <span
            ref={domRef}
            className="bg-sentio-gray-50 border-border-color inline-block w-full cursor-pointer rounded border px-2 py-0.5"
          >
            <ContractAddress address={address} />
            <span className="text-primary-800">.</span>
            <PopoverTooltip
              hideArrow
              className="!inline-flex"
              strategy="fixed"
              maxWidth="max-w-[60vw]"
              text={
                <div
                  className="text-ilabel w-fit space-y-2 text-gray-800"
                  onClick={(evt) => {
                    evt.stopPropagation()
                  }}
                >
                  <h3 className="text-ilabel text-primary-800/70 font-semibold">
                    Function Name/Signature:
                  </h3>
                  <CopyButton text={name || topics[0]}>
                    <span className="text-magenta/70 dark:text-magenta-800">
                      {name || topics[0]}
                    </span>
                  </CopyButton>
                  {topics?.length > 0 ? (
                    <div className="space-y-2">
                      <h3 className="text-ilabel text-primary-800/70 font-semibold">
                        Topics
                      </h3>
                      {topics?.map((topic, index) => (
                        <div key={index} className="flex items-center gap-2">
                          <span className="text-gray-600">[{index}]</span>
                          <span>{topic}</span>
                        </div>
                      ))}
                    </div>
                  ) : null}
                </div>
              }
            >
              <span className="text-magenta/70 dark:text-magenta-800">
                {name || topics[0]}
              </span>
            </PopoverTooltip>
            <span className="text-gray mx-1">
              <span>(</span>
              {events ? (
                events?.map((event, index) => (
                  <ContractParam
                    data={event}
                    key={`${event.name}_${index}`}
                    isLast={index === events.length - 1}
                  />
                ))
              ) : (
                <ContractParam
                  isLast
                  data={{
                    value: rawData,
                    type: 'bytes'
                  }}
                />
              )}
              <span>)</span>
            </span>
          </span>
        </div>
      </div>
      {supportToDebug && (
        <div
          className={cx(
            'absolute -left-11 -top-0.5 px-1 py-1.5',
            isSelected ? '' : 'z-[1] hidden group-hover/tree:block'
          )}
        >
          <PopoverTooltip
            className=""
            hideArrow
            text={
              <span className="dark:bg-sentio-gray-100 z-[1] bg-white">
                Open In Debugger
              </span>
            }
            strategy="fixed"
            icon={
              <span
                className="bg-primary hover:bg-primary-500 active:bg-primary-700 inline-block h-4 w-4 cursor-pointer rounded-sm"
                onClick={toDebug}
              >
                <ContractDebugIcon className="h-4 w-4" />
              </span>
            }
          ></PopoverTooltip>
        </div>
      )}
    </div>
  )
})

interface StorageNodeProps {
  data: ExtendedStorage
  depth?: number
  editorNode?: React.ReactNode
  expander?: number
}

const StorageNode = memo(function StorageNode({
  data,
  ...params
}: StorageNodeProps) {
  const { wkey, address, slot, value, decodedVariable } =
    data as ExtendedStorage
  let depth = 0
  if (wkey) {
    depth = wkey?.split('.')?.length ? wkey.split('.').length - 1 : 0
  }

  return (
    <div className="relative font-mono">
      <div className="flex">
        <div className={opLabelClass({ status: 'storage' })}>
          <span>{depth}</span>
          <ArrowRightIcon className="h-4 w-4" />
          <span>{data.type}</span>
        </div>
        <div className="inline-flex w-full items-center gap-4">
          <span className="bg-sentio-gray-50 border-border-color inline-block w-full cursor-pointer rounded border px-2 py-0.5">
            <ContractAddress address={address} />
            {decodedVariable?.decoded ? (
              <>
                <span className="text-primary-800">.</span>
                <DecodedVariable data={decodedVariable} />
              </>
            ) : (
              <span className="inline-flex items-center gap-2">
                <span className="inline-flex items-center">
                  <span className="mr-1 text-gray-500">[</span>
                  <span className="text-gray-600">{slot}</span>
                </span>
                <span className="text-gray-500">=</span>
                <span className="inline-flex items-center">
                  <CopyableParam value={value} />
                  <span className="text-gray-500">]</span>
                </span>
              </span>
            )}
          </span>
        </div>
      </div>
    </div>
  )
})

interface CallTreeNodeProps {
  data: ExtendedCall
  depth?: number
  editorNode?: React.ReactNode
  expander?: number
  currentKey?: string
  supportToDebug?: boolean
}

export const CallTreeNode = memo(function CallTreeNode({
  data,
  depth = 0,
  currentKey,
  supportToDebug,
  ...params
}: CallTreeNodeProps) {
  const {
    contractName,
    functionName = '0x',
    location,
    type,
    inputs,
    returnValue,
    to,
    depth: dataDepth,
    gasUsed,
    value,
    error,
    revertReason,
    wkey,
    op,
    parentError
  } = data

  // Optional fields that may not exist on ExtendedCall
  const decodedError = (data as any).decodedError
  const rawInput = (data as any).rawInput
  const refund = (data as any).refund
  const storages = (data as any).storages

  const { showGas } = useContext(CallTraceContext)
  const index = location?.instructionIndex || 0
  const chainId = useContext(ChainIdContext)
  const nativeToken = getNativeToken(chainId)

  const domRef = useRef<HTMLDivElement>(null)
  const { routeTo, setMask } = useContext(OverviewContext) || {}
  const toDebug = useCallback(async () => {
    setMask?.(true)
    try {
      setMask?.(false)
      routeTo?.(`debug?trace=${index}`, true)
    } catch {
      setMask?.(false)
    }
  }, [setMask, index, routeTo])

  const { open } = useSubFundflow()
  const openSubFundflow = useCallback(
    (evt: React.MouseEvent<any>) => {
      evt.stopPropagation()
      open(data)
    },
    [data, open]
  )

  const address =
    to ?? parseCompilationId(location?.compilationId)?.address ?? ''

  const isSelected = currentKey === wkey

  const methodSig = rawInput ? rawInput.substring(0, 10) : ''
  const calldataStr = rawInput ? rawInput.substring(10) : ''

  // Check if this call trace has sub fund flow data
  const showSubFundFlowButton = useMemo(
    () => hasSubFundFlow(data, chainId),
    [data, chainId]
  )
  const nodeDepth = dataDepth ?? depth

  return (
    <div className="relative overflow-visible font-mono">
      <div className="flex">
        <div
          className={opLabelClass({
            status: error
              ? 'danger'
              : parentError
                ? 'invalid'
                : type?.toLowerCase().includes('call')
                  ? 'external'
                  : 'normal'
          })}
        >
          <span>{nodeDepth}</span>
          <ArrowRightIcon className="h-4 w-4" />
          <span>{type ?? op}</span>
        </div>
        {error ? (
          <>
            <span
              ref={domRef}
              className="bg-sentio-gray-50 border-border-color mr-2 inline-block w-fit cursor-pointer rounded border px-2 py-0.5"
            >
              <span className="text-red-800">{upperFirst(error)}</span>
              {revertReason ? (
                <span className="ml-2 text-red-800">({revertReason})</span>
              ) : null}
            </span>
            {decodedError ? (
              <span className="border-border-color mr-2 inline-block w-fit cursor-pointer rounded border bg-red-100/50 px-2 py-0.5">
                <span className="text-red-800">{decodedError?.name}</span>
                <span className="text-gray mx-1">
                  <span>(</span>
                  {decodedError.inputs?.map((item: any, index: number) => {
                    if (item.type !== undefined) {
                      return (
                        <ContractParam
                          data={item}
                          key={`${item.name}_${index}`}
                          isLast={index === decodedError.inputs.length - 1}
                        />
                      )
                    }
                    const rawValue =
                      item?.value !== undefined ? item.value : item
                    return (
                      <span
                        key={index}
                        className={cx(
                          index === decodedError.inputs.length - 1
                            ? ''
                            : 'mr-1',
                          'text-gray'
                        )}
                      >
                        <RawParam data={rawValue} />
                        {index === decodedError.inputs.length - 1 ? null : (
                          <span>, </span>
                        )}
                      </span>
                    )
                  })}
                  <span>)</span>
                </span>
              </span>
            ) : null}
          </>
        ) : null}
        <div className="inline-flex w-full items-center gap-2">
          {showGas && gasUsed && !isZeroValue(gasUsed) ? (
            <span
              className="bg-sentio-gray-50 dark:text-text-foreground border-border-color inline-block rounded border px-2 py-0.5 text-gray-500"
              title="Gas used"
            >
              {displayBigInt(gasUsed)}
            </span>
          ) : null}
          {value && value !== '0x0' && value !== '0x' ? (
            <span className="text-purple bg-sentio-gray-50 border-border-color inline-block rounded border px-2 py-0.5 dark:text-purple-700">
              value: {getNumberWithDecimal(value, nativeToken.tokenDecimals)}{' '}
              {nativeToken.tokenSymbol}
            </span>
          ) : null}
          {showGas && refund && !isZeroValue(refund) ? (
            <span
              className="bg-sentio-gray-50 dark:text-text-foreground border-border-color inline-block rounded border px-2 py-0.5 text-gray-500"
              title="Gas refund"
            >
              Refund: {displayBigInt(refund)}
            </span>
          ) : null}
          <span
            ref={domRef}
            className="bg-sentio-gray-50 border-border-color inline-block w-full cursor-pointer rounded border px-2 py-0.5"
          >
            <ContractAddress
              address={address}
              fallbackName={contractName}
              linkParam="?t=1"
            />
            <span className="text-primary-800">.</span>
            <PopoverTooltip
              hideArrow
              className="!inline-flex"
              strategy="fixed"
              text={
                <div
                  className="text-ilabel w-fit space-y-2"
                  onClick={(evt) => {
                    evt.stopPropagation()
                  }}
                >
                  <div className="text-magenta/70 dark:text-magenta-800 inline-flex items-center">
                    {functionName}
                    <CopyButton
                      text={functionName}
                      className="text-gray ml-2 !inline !h-4 !w-4"
                    />
                  </div>
                  {methodSig && (
                    <div className="relative flex items-center gap-2 font-mono text-gray-800">
                      {methodSig}
                      <CopyButton
                        text={methodSig}
                        className="text-gray !h-4 !w-4"
                      />
                    </div>
                  )}
                  {calldataStr && (
                    <DisclosurePanel
                      title={
                        <div className="flex min-w-[160px] items-center gap-2">
                          Call Data
                          <div onClick={(evt) => evt.stopPropagation()}>
                            <CopyButton
                              text={calldataStr}
                              className="text-gray !h-4 !w-4"
                            />
                          </div>
                        </div>
                      }
                    >
                      <div
                        className="text-gray max-h-[300px] overflow-auto whitespace-pre-wrap break-all"
                        onClick={(evt) => {
                          evt.stopPropagation()
                        }}
                      >
                        {calldataStr}
                      </div>
                    </DisclosurePanel>
                  )}
                </div>
              }
            >
              <span className="text-magenta/70 dark:text-magenta-800">
                {functionName}
              </span>
            </PopoverTooltip>

            <span className="text-gray mx-1">
              <span>(</span>
              {inputs?.map((item: any, index: number) => {
                if (item.type !== undefined) {
                  return (
                    <ContractParam
                      data={item}
                      key={`${item.name}_${index}`}
                      isLast={index === inputs.length - 1}
                    />
                  )
                }
                const rawValue = item?.value !== undefined ? item.value : item
                return (
                  <span
                    key={index}
                    className={cx(
                      index === inputs.length - 1 ? '' : 'mr-1',
                      'text-gray'
                    )}
                  >
                    <RawParam data={rawValue} />
                    {index === inputs.length - 1 ? null : <span>, </span>}
                  </span>
                )
              })}
              <span>)</span>
            </span>
            {isArray(returnValue) && returnValue?.length > 0 ? (
              <>
                <ContractRightTriangleIcon className="text-gray mr-2 inline h-4 w-4" />
                <span className="text-gray">
                  <span>(</span>
                  {returnValue?.map((item: any, index: number) => {
                    return (
                      <span
                        key={index}
                        className={
                          index === returnValue.length - 1 ? '' : 'mr-2'
                        }
                      >
                        <ContractParam
                          data={item}
                          isLast={index === returnValue.length - 1}
                          showRaw
                        />
                      </span>
                    )
                  })}
                  <span>)</span>
                </span>
              </>
            ) : null}
            {isString(returnValue) ? (
              <>
                <ContractRightTriangleIcon className="text-gray mr-2 inline h-4 w-4" />
                <span className="text-gray">
                  <span>(</span>
                  <CopyableParam value={returnValue.toString()} />
                  <span>)</span>
                </span>
              </>
            ) : null}
          </span>
        </div>
      </div>
      <div
        className={cx(
          'absolute -top-0.5 px-1 py-1.5',
          showSubFundFlowButton && dataDepth > 1 ? '-left-[68px]' : '-left-11',
          isSelected ? '' : 'z-[1] hidden group-hover/tree:flex',
          'flex items-center gap-2'
        )}
      >
        {showSubFundFlowButton && dataDepth < 2 ? (
          <PopoverTooltip
            className=""
            hideArrow
            placementOption="right-start"
            text={
              <div className="text-icontent z-[1] flex flex-col gap-2">
                <button
                  onClick={openSubFundflow}
                  className="rounded bg-purple-600 px-2 py-1 text-white hover:bg-purple-500 active:bg-purple-700"
                >
                  <LuRoute className="mr-2 inline-block h-3.5 w-3.5 text-white" />
                  Open Sub Fundflow
                </button>
                {supportToDebug && (
                  <button
                    onClick={toDebug}
                    className="bg-primary hover:bg-primary-500 active:bg-primary-700 rounded px-2 py-1 text-white"
                  >
                    <ContractDebugIcon className="mr-2 inline-block h-4 w-4 text-white" />
                    Open In Debugger
                  </button>
                )}
              </div>
            }
            strategy="fixed"
            icon={
              <span
                className="grid h-4 w-4 cursor-pointer items-center justify-items-center rounded-sm bg-purple-600 hover:bg-purple-500 active:bg-purple-700"
                onClick={openSubFundflow}
              >
                <LuRoute className="h-3 w-3 text-white" />
              </span>
            }
          ></PopoverTooltip>
        ) : (
          <>
            {showSubFundFlowButton && (
              <PopoverTooltip
                className=""
                hideArrow
                text={
                  <span className="dark:bg-sentio-gray-100 z-[1] bg-white">
                    View Sub Fund Flow
                  </span>
                }
                strategy="fixed"
                icon={
                  <span
                    className="grid h-4 w-4 cursor-pointer items-center justify-items-center rounded-sm bg-purple-600 hover:bg-purple-500 active:bg-purple-700"
                    onClick={openSubFundflow}
                  >
                    <LuRoute className="h-3 w-3 text-white" />
                  </span>
                }
              ></PopoverTooltip>
            )}
            {supportToDebug && (
              <PopoverTooltip
                className=""
                hideArrow
                text={
                  <span className="dark:bg-sentio-gray-100 z-[1] bg-white">
                    Open In Debugger
                  </span>
                }
                strategy="fixed"
                icon={
                  <span
                    className="bg-primary hover:bg-primary-500 active:bg-primary-700 inline-block h-4 w-4 cursor-pointer rounded-sm"
                    onClick={toDebug}
                  >
                    <ContractDebugIcon className="h-4 w-4" />
                  </span>
                }
              ></PopoverTooltip>
            )}
          </>
        )}
      </div>
    </div>
  )
})

const CallTreeRoot = (props: CallTreeNodeProps) => {
  const { data: trace } = props

  return (
    <span className="space-x-2">
      <span className="bg-lake-blue-50 border-lake-blue-300 inline-block rounded border px-2 py-0.5 text-orange-700">
        [Sender]
      </span>
      <span className="bg-sentio-gray-50 border-sentio-gray-300 text-primary-800 inline-block rounded border px-2 py-0.5 font-mono">
        <ContractAddress address={(trace as any).from} />
      </span>
    </span>
  )
}

interface CallTraceTreeProps {
  data?: DecodedCallTrace
  onInstruction?: (
    key?: string,
    location?: LocationWithInstructionIndex,
    defLocation?: LocationWithInstructionIndex
  ) => void
  currentCallTraceKey?: string
  editorNode?: React.ReactNode
  gasUsed?: boolean
  showStorage?: boolean
  expander?: number // expander depth
  virtual?: boolean
  height?: CSSProperties['height']
  transaction?: Transaction
  supportToDebug?: boolean
}

export const FlatCallTraceTree = ({
  data,
  onInstruction: _onInstruction,
  editorNode,
  gasUsed,
  showStorage,
  expander,
  virtual,
  height,
  transaction,
  supportToDebug
}: CallTraceTreeProps) => {
  const [selectedKey, setSelectedKey] = useState<DataNode['key'] | undefined>()
  const [highlightKey, setHighlightKey] = useState<string | undefined>()
  const calltraceContext = useMemo(() => {
    return {
      showGas: gasUsed
    }
  }, [gasUsed])

  const nodes = useMemo(() => {
    const res: DataNode[] = []
    if (!data) {
      return res
    }
    setCallTraceKeys(data as ExtendedCall)
    const dig = (
      item: ExtendedCall | ExtendedLog | ExtendedStorage
    ): DataNode => {
      if ((item as ExtendedStorage).slot) {
        return {
          title: <StorageNode data={item as ExtendedStorage} />,
          key: (item as ExtendedStorage).wkey || '',
          raw: item
        } as DataNode
      }
      if ((item as ExtendedLog).events || (item as ExtendedLog).topics) {
        return {
          title: (
            <LogEventNode
              data={item as ExtendedLog}
              currentKey={selectedKey as string}
              supportToDebug={supportToDebug}
            />
          ),
          key: (item as ExtendedLog).wkey || '',
          raw: item
        } as DataNode
      }
      const { calls = [], logs = [] } = item as ExtendedCall
      const storages = (item as any).storages || []
      const subNodes: (ExtendedStorage | ExtendedLog | ExtendedCall)[] =
        showStorage ? [...storages, ...calls, ...logs] : [...calls, ...logs]
      const children: DataNode[] = sortBy(subNodes, 'startIndex').map(dig)
      return {
        title: (
          <CallTreeNode
            data={item as ExtendedCall}
            currentKey={selectedKey as string}
            supportToDebug={supportToDebug}
          />
        ),
        key: item.wkey || '',
        children,
        raw: item
      } as DataNode
    }
    res.push(dig(data as ExtendedCall))
    return res
  }, [data, showStorage, selectedKey])

  const onClick = useCallback(
    (data: DataNode) => {
      const { key, raw } = data
      if (raw.slot && raw.value) {
        // storage node
        const location = raw.decodedVariable?.location
        if (location) {
          setSelectedKey((v) => (key === v ? undefined : key))
          _onInstruction?.(key as string, location)
        } else {
          setSelectedKey((v) => (key === v ? undefined : key))
          _onInstruction?.(key as string, {} as any)
        }
        return
      }
      const { location, defLocation } = (raw as ExtendedCall) || {}
      if (location) {
        setSelectedKey((v) => (key === v ? undefined : key))
        _onInstruction?.(key as string, location, defLocation)
      }
    },
    [_onInstruction]
  )

  const scrollIntoView = useContext(SvgFolderContext) === ''

  if (!data) {
    return null
  }

  return (
    <FloatingDelayGroup delay={50}>
      <SubFundflowProvider transaction={transaction}>
        <CallTraceContext.Provider value={calltraceContext}>
          <FlatTree
            data={nodes}
            defaultExpandAll
            virtual={virtual}
            rowHeight={32.06}
            height={height}
            onClick={onClick}
            suffixNode={editorNode}
            expandDepth={expander}
            scrollToKey={highlightKey}
            scrollIntoView={scrollIntoView}
          />
        </CallTraceContext.Provider>
      </SubFundflowProvider>
    </FloatingDelayGroup>
  )
}
