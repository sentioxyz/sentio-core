import { ReactNode, useMemo } from 'react'
import { StatusBadge, StatusRole, useMobile } from '@sentio/ui-core'
import { Transaction, Block, TransactionReciept } from './types'
import { getNativeToken } from './ERC20Token'
import { usePrice, getBlockTime } from './use-price'
import { parseHex, getNumberWithDecimal, BD } from '@sentio/ui-core'
import upperFirst from 'lodash/upperFirst'
import multiply from 'lodash/multiply'
import dayjs from 'dayjs'
import utc from 'dayjs/plugin/utc'
import relativeTime from 'dayjs/plugin/relativeTime'
import EtherLink from './EtherLink'
import { ContractAddress } from './ContractComponents'
import { HeaderToolsToggleButton, HeaderToolsContent } from '@sentio/ui-core'
import { useState } from 'react'

dayjs.extend(utc)
dayjs.extend(relativeTime)

// Helper to convert chainId to number
function chainIdToNumber(chainId?: string): number | undefined {
  if (!chainId) return undefined
  if (chainId.includes('_')) return undefined
  if (chainId.startsWith('0x')) return parseInt(chainId, 16)
  return parseInt(chainId)
}

function displayNumber(hex?: string) {
  if (!hex) return null
  try {
    const num = parseHex(hex)
    return num.toLocaleString()
  } catch {
    return hex
  }
}

function formatCurrency(value: number) {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(value)
}

const TransactionStatus = ({ status }: { status?: string }) => {
  switch (status) {
    case '0x0':
      return <StatusBadge status="Failed" role={StatusRole.Error} bubble />
    case '0x1':
      return <StatusBadge status="Success" role={StatusRole.Success} bubble />
    default:
      return <StatusBadge status="Pending" role={StatusRole.Warning} bubble />
  }
}

const TransactionBlock = ({
  blockNumber,
  latestBlockNumber,
  chainId
}: {
  blockNumber?: string
  latestBlockNumber?: string
  chainId?: string
}) => {
  if (!blockNumber) {
    return null
  }
  const block = parseHex(blockNumber)
  return (
    <span className="inline-flex items-center space-x-2">
      <span className="text-text-foreground inline-flex gap-1">
        <span>{block.toString()}</span>
        <span className="inline-flex items-center gap-1">
          <EtherLink
            address={block.toString()}
            chainId={chainId}
            type="block"
            trigger="static"
          />
        </span>
      </span>
      {latestBlockNumber !== undefined ? (
        <span className="text-gray-500">
          ({(parseHex(latestBlockNumber) - block).toLocaleString()} blocks ago)
        </span>
      ) : null}
    </span>
  )
}

const InfoItem = ({
  label,
  value
}: {
  label: string
  value: React.ReactNode
}) => {
  return (
    <span className="inline-flex max-w-full items-center space-x-2">
      <span className="flex-0 inline-flex text-gray-500">{label}:</span>
      <span className="text-text-foreground inline-flex flex-1 overflow-hidden">
        {value}
      </span>
    </span>
  )
}

function displayDate(hex?: string, local = false) {
  if (!hex) {
    return null
  }
  const timestamp = parseHex(hex) * BigInt(1000)
  const time = dayjs(Number(timestamp))
  if (local) {
    return `${time.format('YYYY-MM-DD HH:mm:ss')} (${Intl.DateTimeFormat().resolvedOptions().timeZone})`
  }
  return time.utc().format('YYYY-MM-DD HH:mm:ss (UTC)')
}

function humanizeDate(hex?: string) {
  if (!hex) {
    return null
  }
  const timestamp = parseHex(hex) * BigInt(1000)
  const time = dayjs(Number(timestamp))
  return time.fromNow()
}

function displayTransactionType(hex?: string) {
  if (!hex) {
    return null
  }
  const type = parseHex(hex)
  if (type === BigInt(0)) {
    return '0 (EIP-2718)'
  } else if (type === BigInt(1)) {
    return '1 (EIP-2930)'
  } else if (type === BigInt(2)) {
    return '2 (EIP-1559)'
  }
  return null
}

interface Props {
  transaction: Transaction
  block?: Block
  receipt?: TransactionReciept
  latestBlockNumber?: string
  chainId?: string
  error?: string
  errorReason?: ReactNode
  refund?: string
  simulationId?: string
  // Optional: render simulation-specific content (e.g., OriginalTxn)
  renderSimulationInfo?: (simulationId: string) => ReactNode
}

export const TransactionBrief = ({
  transaction,
  block = {} as Block,
  receipt = {} as TransactionReciept,
  latestBlockNumber,
  chainId,
  error,
  errorReason,
  refund,
  simulationId,
  renderSimulationInfo
}: Props) => {
  const nativeToken = getNativeToken(chainId)
  const [isToolsOpen, setIsToolsOpen] = useState(false)

  const { data: priceData } = usePrice(
    getBlockTime(block?.timestamp),
    nativeToken.priceTokenAddress,
    chainIdToNumber(chainId)
  )

  const txnFee = useMemo(() => {
    return BD(receipt.gasUsed)
      .multipliedBy(BD(transaction.gasPrice))
      .dividedBy(BD(10).pow(nativeToken.tokenDecimals))
  }, [receipt.gasUsed, transaction.gasPrice, nativeToken.tokenDecimals])

  const amountEther = txnFee.isZero() ? 0 : txnFee.toNumber()
  const isMobile = useMobile()
  const transactionFee = txnFee.isZero()
    ? `0 ${nativeToken.tokenSymbol}`
    : `${isMobile ? txnFee.toExponential(3) : txnFee.toString()} ${nativeToken.tokenSymbol}`

  const lessImportantInfo = (
    <>
      <InfoItem label="Gas limit" value={displayNumber(transaction.gas)} />
      <InfoItem label="Gas used" value={displayNumber(receipt.gasUsed)} />
      {refund && <InfoItem label="Gas refund" value={displayNumber(refund)} />}
      <InfoItem
        label="Gas price"
        value={`${getNumberWithDecimal(transaction.gasPrice, 9)} Gwei`}
      />
      {block.baseFeePerGas !== undefined ? (
        <InfoItem
          label="Base"
          value={`${getNumberWithDecimal(block.baseFeePerGas, 9)} Gwei`}
        />
      ) : null}
      {transaction.maxFeePerGas !== undefined ? (
        <InfoItem
          label="Max"
          value={`${getNumberWithDecimal(transaction.maxFeePerGas, 9)} Gwei`}
        />
      ) : null}
      {transaction.maxPriorityFeePerGas !== undefined ? (
        <InfoItem
          label="Max Priority"
          value={`${getNumberWithDecimal(transaction.maxPriorityFeePerGas, 9)} Gwei`}
        />
      ) : null}
      <InfoItem
        label="Txn Type"
        value={displayTransactionType(transaction.type)}
      />
    </>
  )

  return (
    <>
      <div className="flex flex-wrap gap-x-5 gap-y-2 py-3">
        <InfoItem
          label="Status"
          value={<TransactionStatus status={receipt.status} />}
        />
        {error ? (
          <InfoItem
            label="Error Reason"
            value={
              <>
                <span className="text-red font-medium">
                  {upperFirst(error)}
                </span>
                {errorReason}
              </>
            }
          />
        ) : null}
        <InfoItem
          label="Timestamp"
          value={
            <span
              title={displayDate(block.timestamp) || ''}
              className="whitespace-nowrap"
            >
              {block.timestamp ? displayDate(block.timestamp, true) : null}
              <span className="text-gray-500">
                {block.timestamp ? ` (${humanizeDate(block.timestamp)})` : null}
              </span>
            </span>
          }
        />
        <InfoItem
          label="Block"
          value={
            <TransactionBlock
              blockNumber={block.number}
              latestBlockNumber={latestBlockNumber}
              chainId={chainId}
            />
          }
        />
        <InfoItem
          label="Position In Block"
          value={displayNumber(transaction.transactionIndex)}
        />
        <InfoItem
          label="Transaction Fee"
          value={`${transactionFee} ${
            amountEther && priceData?.price
              ? `(${formatCurrency(multiply(amountEther, priceData.price))} USD)`
              : ''
          }`}
        />
        {!isMobile && lessImportantInfo}
        {isMobile && (
          <HeaderToolsToggleButton
            isOpen={isToolsOpen}
            onClick={() => setIsToolsOpen(!isToolsOpen)}
          />
        )}
        {simulationId && renderSimulationInfo
          ? renderSimulationInfo(simulationId)
          : null}
      </div>
      {isMobile && (
        <HeaderToolsContent isOpen={isToolsOpen}>
          <div className="my-4 flex flex-wrap gap-x-5 gap-y-2">
            {lessImportantInfo}
          </div>
        </HeaderToolsContent>
      )}
      <div className="flex flex-wrap gap-x-5 gap-y-2 overflow-hidden py-3">
        <InfoItem
          label="Sender"
          value={
            <ContractAddress address={transaction.from} showAvatar noPrefix />
          }
        />
        <InfoItem label="Nonce" value={displayNumber(transaction.nonce)} />
        {transaction.to && (
          <InfoItem
            label="Receiver"
            value={
              <ContractAddress address={transaction.to} showAvatar noPrefix />
            }
          />
        )}
      </div>
    </>
  )
}
