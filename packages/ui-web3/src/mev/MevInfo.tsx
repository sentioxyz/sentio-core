/* eslint-disable react/jsx-no-target-blank */
import { Fragment, useEffect, useMemo } from 'react'
import { cx as classNames } from 'class-variance-authority'
import { ArrowsRightLeftIcon, LightBulbIcon } from '@heroicons/react/24/outline'
import { useResizeDetector } from 'react-resize-detector'
import { RiTokenSwapFill } from 'react-icons/ri'
import { MevLink } from './MevLink'
import { FlashbotIcon } from './icons/FlashbotIcon'
import { OneInchIcon } from './icons/1inch'
import {
  UniswapIcon,
  PancakeswapIcon,
  SushiswapIcon,
  CurveIcon,
  AnySwapIcon,
  SnowswapIcon
} from './icons/SwapIcons'
import { OkxIcon } from './icons/Okx'
import { SandwichTxns, SandwichResult } from './SandwichTxns'

export enum MevType {
  SANDWICH = 'sandwich',
  ARBITRAGE = 'arbitrage',
  UNKNOWN = 'unknown'
}

export interface Token {
  address?: string
  symbol?: string
}

export interface Trader {
  address?: string
  protocol?: string
  tokens?: Token[]
}

export interface Revenue {
  totalUsd?: number
}

export interface ArbitrageResult {
  txHash?: string
  txIndex?: number
  revenues?: Revenue
  tokens?: Token[]
  traders?: Trader[]
}

export interface MevData {
  type?: 'VICTIM' | 'ATTACKER' | 'NONE'
  sandwich?: SandwichResult & {
    revenues?: Revenue
    tokens?: Token[]
    traders?: Trader[]
  }
  arbitrage?: ArbitrageResult
  txIndex?: number
  blockNumber?: string
}

const swapIcons: Record<string, React.ReactNode> = {
  uniswap: <UniswapIcon className="inline-block h-5 w-5" />,
  pancakeswap: <PancakeswapIcon className="inline-block h-5 w-5" />,
  sushiswap: <SushiswapIcon className="inline-block h-5 w-5" />,
  curve: <CurveIcon className="inline-block h-5 w-5" />,
  anyswap: <AnySwapIcon className="inline-block h-5 w-5" />,
  snowswap: <SnowswapIcon className="inline-block h-5 w-5" />
}

const PoolIcon = ({ poolName }: { poolName?: string }) => {
  if (!poolName) {
    return <div className="inline-block h-5 w-5 animate-pulse rounded-full" />
  }

  const name = poolName.toLowerCase()
  let icon = (
    <RiTokenSwapFill className="text-primary/50 inline-block h-5 w-5" />
  )
  Object.keys(swapIcons).forEach((prefix) => {
    if (name.startsWith(prefix)) {
      icon = swapIcons[prefix] as React.ReactElement
    }
  })
  return icon as React.ReactElement
}

export interface MevInfoProps {
  hash?: string
  chainId?: string
  data?: MevData | null
  loading?: boolean
  metamaskBtn?: React.ReactNode
  mevCallback?: (mevType: MevType, role: string, value: string) => void
  /**
   * Custom token renderer. If provided, will be used instead of default ERC20Token component.
   * @param address - Token address
   * @param symbol - Token symbol
   */
  renderToken?: (address: string, symbol?: string) => React.ReactNode
  /**
   * Helper to format currency values. Defaults to simple toString with $ prefix
   */
  formatCurrency?: (value: number) => string
  /**
   * If true, hide arbitrage MEV types. Defaults to true
   */
  hideArbitrage?: boolean
  /**
   * If true, hide sandwich attacker when in extension mode. Defaults to false
   */
  isExtension?: boolean
  className?: string
}

const defaultFormatCurrency = (value: number): string => {
  return '$' + value.toFixed(2)
}

export const MevInfo = ({
  hash,
  chainId = '1',
  data,
  loading = false,
  metamaskBtn,
  mevCallback,
  renderToken,
  formatCurrency = defaultFormatCurrency,
  hideArbitrage = true,
  isExtension = false,
  className
}: MevInfoProps) => {
  const { type, sandwich, arbitrage, txIndex, blockNumber } = data || {}
  const mevRole = type === 'VICTIM' ? 'Victim' : 'Attacker'
  let mevType: MevType = MevType.UNKNOWN
  if (sandwich) {
    mevType = MevType.SANDWICH
  } else if (arbitrage) {
    mevType = MevType.ARBITRAGE
  }
  const { ref, width } = useResizeDetector({ handleHeight: false })
  const isSmallWidth = width && width < 1080

  const value = useMemo(() => {
    if (mevType === MevType.SANDWICH) {
      return sandwich?.revenues?.totalUsd
    }
    if (mevType === MevType.ARBITRAGE) {
      return arbitrage?.revenues?.totalUsd
    }
    return 0
  }, [mevType, sandwich, arbitrage])

  useEffect(() => {
    if (mevType !== MevType.UNKNOWN) {
      mevCallback?.(mevType, mevRole, value ? formatCurrency(value) : '')
    }
  }, [mevType, mevRole, value, mevCallback, formatCurrency])

  const tokens = useMemo(() => {
    if (mevType === MevType.SANDWICH) {
      return sandwich?.tokens
    }
    if (mevType === MevType.ARBITRAGE) {
      return arbitrage?.tokens
    }
    return []
  }, [mevType, sandwich, arbitrage])

  const traders = useMemo(() => {
    if (mevType === MevType.SANDWICH) {
      return sandwich?.traders
    }
    if (mevType === MevType.ARBITRAGE) {
      return arbitrage?.traders
    }
    return []
  }, [mevType, sandwich, arbitrage])

  if ((!data && !loading) || data?.type === 'NONE' || chainId !== '1') {
    return null
  }
  // hide arbitrage
  if (hideArbitrage && mevType === MevType.ARBITRAGE) {
    return null
  }
  // hide sandwich attacker
  if (mevRole === 'Attacker' && mevType === MevType.SANDWICH && isExtension) {
    return null
  }

  if (loading) {
    return (
      <div
        className={classNames(
          'mb-2.5 flex items-center justify-center p-8',
          className
        )}
      >
        <div className="border-t-primary-600 h-8 w-8 animate-spin rounded-full border-4 border-gray-300" />
      </div>
    )
  }

  return (
    <div className={className}>
      <div className="mb-2.5 flex flex-1 items-center gap-2">
        <h3 className="text-gray font-ilabel capitalize">
          MEV {mevType} Attack Detected
        </h3>
        <div className="bg-red/10 text-red rounded-md px-1.5 py-0.5 text-xs font-semibold">
          MEV
        </div>
      </div>
      <div
        className={classNames(
          'text-ialbel flex w-full flex-wrap items-stretch',
          isSmallWidth ? 'gap-4' : 'flex-nowrap gap-6',
          'rounded-lg border p-4'
        )}
        ref={ref}
      >
        <div className="flex-1 basis-3/4">
          <div className="flex w-full flex-wrap items-center gap-6 border-b pb-4">
            <div className="inline-flex items-center space-x-2">
              <span className="text-gray font-medium">
                Estimated Value Loss:
              </span>
              {value === undefined ? null : (
                <span
                  className={classNames(
                    'font-mono font-semibold',
                    value > 0 ? 'text-red-500' : 'text-cyan'
                  )}
                >
                  {formatCurrency(value)}
                </span>
              )}
            </div>
            <div className="inline-flex items-center gap-4">
              <div className="text-gray flex-0 font-medium">Tokens:</div>
              {tokens?.map((item) => (
                <Fragment key={item.address}>
                  {item.address && renderToken
                    ? renderToken(item.address, item.symbol)
                    : item.address && (
                        <span className="font-mono text-xs">
                          {item.symbol ||
                            `${item.address.slice(0, 6)}...${item.address.slice(-4)}`}
                        </span>
                      )}
                </Fragment>
              ))}
            </div>
          </div>
          {traders && traders?.length > 0 ? (
            <div className="space-y-1 border-b py-4">
              <h3 className="text-gray font-medium">Pools Used:</h3>
              <div>
                {traders?.map((traderItem) => {
                  if (!traderItem.address || !traderItem.protocol) {
                    return <Fragment key={traderItem.address}></Fragment>
                  }
                  return (
                    <div
                      key={traderItem.address}
                      className="grid w-full grid-cols-3 gap-2 py-1"
                    >
                      <div className="inline-flex items-center gap-2">
                        <PoolIcon poolName={traderItem.protocol} />
                        <span className="text-primary-800/80 whitespace-nowrap">
                          {traderItem.protocol}
                        </span>
                      </div>
                      <div className="text-center">
                        <MevLink
                          data={traderItem.address}
                          type="address"
                          className="text-gray text-xs"
                          truncate
                          chainId={chainId}
                        />
                      </div>
                      <div className="text-text-foreground flex w-full justify-end gap-4">
                        {traderItem.tokens?.map((item, index) =>
                          item.address ? (
                            <Fragment key={item.address}>
                              {renderToken ? (
                                renderToken(item.address, item.symbol)
                              ) : (
                                <span className="font-mono text-xs">
                                  {item.symbol ||
                                    `${item.address.slice(0, 6)}...${item.address.slice(-4)}`}
                                </span>
                              )}
                              {index < traderItem.tokens!.length - 1 ? (
                                <ArrowsRightLeftIcon className="text-gray h-4 w-4" />
                              ) : null}
                            </Fragment>
                          ) : null
                        )}
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          ) : null}
          <div
            className={classNames(
              'space-y-2 pt-4',
              isSmallWidth ? 'border-b pb-4' : ''
            )}
          >
            <div className="flex w-full items-center justify-between">
              <h3 className="text-gray font-medium">MEV Replay:</h3>
              {blockNumber ? (
                <div className="text-gray space-x-1">
                  <span className="font-medium">Block:</span>
                  <MevLink
                    data={blockNumber}
                    type="block"
                    trigger="static"
                    truncate
                    chainId={chainId}
                  />
                </div>
              ) : null}
            </div>
            <div className="text-gray">
              {mevType === MevType.SANDWICH ? (
                <SandwichTxns
                  data={sandwich!}
                  currentTxHash={hash!}
                  chainId={chainId}
                />
              ) : null}
              {mevType === MevType.ARBITRAGE ? (
                <div className="w-full">
                  {type === 'VICTIM' && hash ? (
                    <div className="grid w-full grid-cols-3 gap-2 py-1 ">
                      <span className="w-20 font-medium">Victim:</span>
                      <span className="text-center">
                        <MevLink
                          data={hash}
                          type="tx"
                          truncate
                          chainId={chainId}
                        />
                      </span>
                      <span className="min-w-20 whitespace-nowrap text-right">
                        Position: {txIndex}
                      </span>
                    </div>
                  ) : null}
                  {arbitrage?.txHash ? (
                    <div className="grid w-full grid-cols-3 gap-2 py-1 ">
                      <span className="w-36 font-medium">Arbitrage:</span>
                      <span className="text-center">
                        <MevLink
                          data={arbitrage.txHash}
                          type="tx"
                          truncate
                          chainId={chainId}
                        />
                      </span>
                      <span className="min-w-20 whitespace-nowrap text-right">
                        Position: {arbitrage?.txIndex}
                      </span>
                    </div>
                  ) : null}
                </div>
              ) : null}
            </div>
          </div>
        </div>
        {isSmallWidth ? null : (
          <div className="flex-0 border-border-color my-2 border-l"></div>
        )}
        <div
          className={classNames(
            isSmallWidth ? 'w-full' : 'w-1/3',
            'flex-0 space-y-4'
          )}
        >
          <h3 className="text-gray font-ilabel">Recommended Action</h3>
          <ul className="space-y-2">
            {mevType === MevType.SANDWICH ? (
              <li className="text-gray space-x-2">
                <LightBulbIcon className="text-primary inline-block h-4 w-4 align-text-bottom" />
                <span>
                  Use a{' '}
                  <a
                    href="https://docs.flashbots.net/flashbots-protect/overview"
                    target="_blank"
                    rel="noreferrer"
                    className="text-primary font-semibold hover:underline"
                  >
                    MEV-protected
                  </a>{' '}
                  endpoint, e.g.{' '}
                </span>
                {metamaskBtn ?? (
                  <a
                    href="https://docs.flashbots.net/flashbots-protect/quick-start"
                    target="_blank"
                    className="text-primary hover:underline"
                  >
                    <FlashbotIcon className="mr-1 inline-block h-4 w-4 align-text-bottom" />
                    Flashbot RPC
                  </a>
                )}
              </li>
            ) : null}
            {mevType === MevType.ARBITRAGE ? (
              <li className="text-gray space-x-2">
                <LightBulbIcon className="text-primary inline-block h-4 w-4 align-text-bottom" />
                <span>Use a DEX aggregator, e.g. </span>
                <a
                  href="https://app.1inch.io/"
                  target="_blank"
                  className="text-primary hover:underline"
                >
                  <OneInchIcon className="mr-1 inline-block h-4 w-4 align-text-bottom" />
                  1inch
                </a>
                <span>or</span>
                <a
                  href="https://www.okx.com/web3/dex-swap"
                  target="_blank"
                  className="text-primary hover:underline"
                >
                  <OkxIcon className="mr-1 inline-block h-4 w-4 align-text-bottom" />
                  OKX
                </a>
              </li>
            ) : null}
          </ul>
        </div>
      </div>
    </div>
  )
}
