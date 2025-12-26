import { sentioProjectSimUrl, sentioSimUrl, sentioTxUrl } from '~/utils/url'
import { SimulateSlideOver } from '../simulate/SimulateSlideover'
import { CallTracePanel } from './CallTracePanel'
import { FundFlowPanel } from './FundFlowPanel'
import { useEffect, useState, useContext, useMemo } from 'react'
import { Transition } from '@headlessui/react'
import { useCallTrace } from '~/content/lib/debug/use-call-trace'
import { BalanceChangePanel, GetPriceRequest } from './BalanceChangePanel'
import { useTransactionInfo } from '~/content/lib/debug/use-transaction-info'
import { getChainExternalUrl } from '@sentio/chain'
import { IsSimulationContext } from '~/content/lib/context/transaction'
import { MevInfo } from '../mev/MevInfo'
import {
  TransactionBrief,
  SimulatorInfo,
  SpinLoading,
  SvgFolderContext,
  OpenContractContext,
  SenderContext,
  ReceiverContext,
  ChainIdContext,
  PriceFetcherContext,
  classNames,
  parseNamesFromTraceData,
  TagCacheProvider
} from '@sentio/ui-web3'
import { useSimulator } from '~/content/lib/debug/use-simulator'
import { TagProvider } from './TagProvider'

async function priceFetcher(params: GetPriceRequest) {
  const res = await chrome.runtime.sendMessage({
    api: 'GetPrice',
    data: params
  })
  return res
}

interface Props {
  hash: string
  chainId: string
  defaultShowCallTrace?: boolean
  defaultShowBalanceChange?: boolean
  showBrief?: boolean
  projectOwner?: string
  projectSlug?: string
}

const MenuOpenIcon = (props) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    fill="none"
    viewBox="0 0 24 24"
    strokeWidth="1.5"
    stroke="currentColor"
    className="h-6 w-6"
    {...props}
  >
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      d="M19.5 5.25l-7.5 7.5-7.5-7.5m15 6l-7.5 7.5-7.5-7.5"
    />
  </svg>
)

export const TransactionCard = ({
  hash,
  chainId,
  defaultShowCallTrace = false,
  defaultShowBalanceChange = true,
  showBrief,
  projectOwner,
  projectSlug
}: Props) => {
  const [isOpen, setIsOpen] = useState(defaultShowCallTrace)
  const [isBalanceOpen, setIsBalanceOpen] = useState(defaultShowBalanceChange)
  const callTraceRes = useCallTrace(hash, chainId)
  const { data: callTrace, loading: callTraceLoading } = callTraceRes
  const { data: allCallTraces } = useCallTrace(hash, chainId, true)
  const { data: transaction, loading: transactionLoading } = useTransactionInfo(
    hash,
    chainId
  )
  const isSimulation = useContext(IsSimulationContext)

  useEffect(() => {
    if (allCallTraces) {
      parseNamesFromTraceData(allCallTraces)
    }
  }, [allCallTraces])

  const sentioLink = useMemo(() => {
    if (projectOwner && projectSlug) {
      return isSimulation
        ? sentioProjectSimUrl(chainId, hash, projectOwner, projectSlug)
        : sentioTxUrl(chainId, hash)
    }
    return isSimulation
      ? sentioSimUrl(chainId, hash)
      : sentioTxUrl(chainId, hash)
  }, [isSimulation, chainId, hash, projectOwner, projectSlug])

  const { data: simData } = useSimulator(isSimulation ? hash : '')

  return (
    <TagCacheProvider>
      <SvgFolderContext.Provider value="https://app.sentio.xyz/">
        <SpinLoading
          loading={transactionLoading}
          className={transactionLoading ? 'min-h-[300px]' : ''}
        >
          {transaction?.block === null ? (
            <div className="text-center text-gray-400">
              The pending transaction does not have a fund flow.
            </div>
          ) : (
            transaction && (
              <PriceFetcherContext.Provider value={priceFetcher}>
                <OpenContractContext.Provider
                  value={(address: string, chain: string) => {
                    const targetLink = getChainExternalUrl(
                      chain,
                      address,
                      'address'
                    )
                    if (targetLink) {
                      window.open(targetLink, '_blank')
                    }
                  }}
                >
                  <SenderContext.Provider
                    value={transaction?.transaction?.from}
                  >
                    <ReceiverContext.Provider
                      value={transaction?.transaction?.to}
                    >
                      <ChainIdContext.Provider value={chainId}>
                        <div className="space-y-2">
                          {showBrief ? (
                            <div className="divide-y border-b">
                              <TransactionBrief
                                transaction={transaction?.transaction}
                                block={transaction?.block}
                                chainId={chainId}
                                receipt={transaction?.transactionReceipt}
                                latestBlockNumber={
                                  transaction?.latestBlockNumber
                                }
                              />
                              {isSimulation ? (
                                <SimulatorInfo
                                  simulationData={simData}
                                  className="py-2"
                                />
                              ) : null}
                            </div>
                          ) : null}
                          <div>
                            <div
                              className="rounded-md bg-gray-50 text-center hover:bg-gray-100"
                              onClick={() => {
                                setIsBalanceOpen(!isBalanceOpen)
                              }}
                            >
                              <button className="text-gray px-2 py-1 text-[12px]">
                                <MenuOpenIcon
                                  className={classNames(
                                    'text-gray mr-2 inline h-4 w-4 align-text-top transition-transform',
                                    isBalanceOpen ? 'rotate-180' : ''
                                  )}
                                />
                                {isBalanceOpen
                                  ? 'Collapse Balance Change'
                                  : 'Expand Balance Change'}
                              </button>
                            </div>
                            <Transition
                              show={isBalanceOpen}
                              enter="transition ease-out duration-200"
                              enterFrom="transform opacity-0 scale-95"
                              enterTo="transform opacity-100 scale-100"
                              leave="transition ease-in duration-75"
                              leaveFrom="transform opacity-100 scale-100"
                              leaveTo="transform opacity-0 scale-95"
                              unmount={false}
                            >
                              <div className="relative space-y-2 pt-4">
                                <div className="text-gray font-ilabel">
                                  Balance Change
                                </div>
                                <div className="rounded-lg border px-2 pt-4">
                                  <BalanceChangePanel
                                    transaction={transaction}
                                    loading={callTraceRes.loading}
                                    data={callTraceRes.data}
                                  />
                                </div>
                              </div>
                            </Transition>
                          </div>
                          <div className="relative pb-2">
                            <div className="flex w-full justify-between">
                              <div className="text-gray font-ilabel">
                                Fund Flow
                              </div>
                              <span className="inline-flex items-center gap-2">
                                <SimulateSlideOver
                                  hash={hash}
                                  chainId={chainId}
                                />
                                <a
                                  target="_blank"
                                  href={sentioLink}
                                  rel="nofollow noreferrer"
                                  className="text-icontent text-gray hover:border-primary hover:text-primary rounded border px-2 py-0.5"
                                >
                                  Open in Sentio
                                </a>
                              </span>
                            </div>
                            <div className="mt-2 rounded-lg border p-4">
                              <FundFlowPanel
                                hash={hash}
                                chainId={chainId}
                                allCallTraces={allCallTraces}
                                callTrace={callTrace}
                                callTraceLoading={callTraceLoading}
                                transaction={transaction}
                                transactionLoading={transactionLoading}
                              />
                            </div>
                          </div>
                          <div>
                            <MevInfo hash={hash} chainId={chainId} />
                          </div>
                          <div>
                            <div
                              className="rounded-md bg-gray-50 text-center hover:bg-gray-100"
                              onClick={() => {
                                setIsOpen(!isOpen)
                              }}
                            >
                              <button className="text-gray px-2 py-1 text-[12px]">
                                <MenuOpenIcon
                                  className={classNames(
                                    'text-gray mr-2 inline h-4 w-4 align-text-top transition-transform',
                                    isOpen ? 'rotate-180' : ''
                                  )}
                                />
                                {isOpen
                                  ? 'Collapse Call Trace'
                                  : 'Expand Call Trace'}
                              </button>
                            </div>
                            <Transition
                              show={isOpen}
                              enter="transition ease-out duration-200"
                              enterFrom="transform opacity-0 scale-95"
                              enterTo="transform opacity-100 scale-100"
                              leave="transition ease-in duration-75"
                              leaveFrom="transform opacity-100 scale-100"
                              leaveTo="transform opacity-0 scale-95"
                              unmount={false}
                            >
                              <div className="relative space-y-2 pt-4">
                                <div className="text-gray font-ilabel pl-2">
                                  Call Trace
                                </div>
                                <CallTracePanel hash={hash} chainId={chainId} />
                              </div>
                            </Transition>
                          </div>
                        </div>
                        <TagProvider
                          callTrace={allCallTraces}
                          chain={chainId}
                          key={`${chainId}/${hash}`}
                        />
                      </ChainIdContext.Provider>
                    </ReceiverContext.Provider>
                  </SenderContext.Provider>
                </OpenContractContext.Provider>
              </PriceFetcherContext.Provider>
            )
          )}
        </SpinLoading>
      </SvgFolderContext.Provider>
    </TagCacheProvider>
  )
}
