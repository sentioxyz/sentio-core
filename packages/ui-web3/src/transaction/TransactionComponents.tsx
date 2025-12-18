import { HexNumber } from './HexNumber'
import { CheckIcon, ClockIcon, XMarkIcon } from '@heroicons/react/24/outline'
import { getNumberWithDecimal } from '@sentio/ui-core'
import { ContractFromIcon, ContractToIcon, InternalIcon } from './Icons'
import { NetworkNode, getNativeToken } from './ERC20Token'
import { CopyButton } from '@sentio/ui-core'

export const TransactionStatus = ({ status }: { status: number }) => {
  if (status === 1) {
    return (
      <span className="text-cyan inline-flex items-center justify-end font-medium">
        <CheckIcon className="mr-2 inline-block h-4 w-4" />
        Success
      </span>
    )
  }
  if (status === 0) {
    return (
      <span className="inline-flex items-center justify-end font-medium text-red-600">
        <XMarkIcon className="mr-2 inline-block h-4 w-4" />
        Failed
      </span>
    )
  }

  return (
    <span className="text-text-foreground-secondary inline-flex items-center justify-end font-medium">
      <ClockIcon className="mr-2 inline-block h-4 w-4" />
      Pending
    </span>
  )
}

export const TransactionValue = ({ value, chainId }: { value: string; chainId?: string }) => {
  const nativeToken = getNativeToken(chainId)
  let text = `0 ${nativeToken.tokenSymbol}`
  try {
    text = `${getNumberWithDecimal(value, nativeToken.tokenDecimals) || 0} ${nativeToken.tokenSymbol}`
  } catch {
    // do nothing
  }
  return <span title={text}>{text}</span>
}

export const AddressFrom = ({ address }: { address: string }) => {
  return (
    <span className="text-gray inline-flex w-full items-center space-x-2">
      <ContractFromIcon className="h-3.5 w-3.5 shrink-0" />
      <span className="flex-1 pr-2">
        <HexNumber data={address?.toLowerCase()} truncate={14} copyable noCopyHint className="!font-mono" />
      </span>
    </span>
  )
}

export const AddressTo = ({ address }: { address: string }) => {
  return (
    <span className="text-gray inline-flex w-full items-center space-x-2">
      <ContractToIcon className="h-3.5 w-3.5 shrink-0" />
      <span className="flex-1 pr-2">
        <HexNumber data={address?.toLowerCase()} truncate={14} copyable noCopyHint className="!font-mono" />
      </span>
    </span>
  )
}

export const TransactionLabel = ({ row, getValue }: { row: any; getValue: () => string }) => {
  const { trace } = row.original
  // internal transaction: trace = true
  return (
    <span className="text-primary-800 inline-flex w-full gap-2 pr-6">
      <HexNumber data={getValue() as string} autoTruncate copyable noCopyHint className="flex-1 !font-mono" />
      <span title="Internal Transaction" className="text-primary-500 hover:text-primary-600 flex-0 h-4 w-4 pr-2">
        {trace ? <InternalIcon className="inline-block h-4 w-4" /> : null}
      </span>
    </span>
  )
}

const TransactionColumns: any[] = [
  {
    id: 'hash',
    header: 'Tx Hash',
    accessorKey: 'hash',
    cell: (info: any) => <TransactionLabel row={info.row} getValue={info.getValue} />,
    size: 300,
    enableResizing: false
  },
  {
    id: 'status',
    header: 'Status',
    accessorKey: 'transactionStatus',
    cell: (info: any) => <TransactionStatus status={info.getValue()} />,
    size: 150,
    enableResizing: false
  },
  {
    id: 'from',
    header: 'From',
    accessorKey: 'tx.from',
    cell: (info: any) => <AddressFrom address={info.getValue()} />,
    size: 200,
    enableResizing: false
  },
  {
    id: 'to',
    header: 'To',
    accessorKey: 'tx.to',
    cell: (info: any) => <AddressTo address={info.getValue()} />,
    size: 200,
    enableResizing: false
  },
  {
    id: 'methodSignature',
    header: 'Method Signature',
    accessorKey: 'methodSignatureText',
    cell: (info: any) => {
      const { methodSignature, methodSignatureText } = info.row.original
      if (methodSignatureText) {
        return (
          <div className="flex h-5 w-full truncate whitespace-nowrap leading-5">
            <span className="flex-1 truncate whitespace-nowrap">{methodSignatureText}</span>
            <span
              className="bg-primary-50 hidden rounded p-0.5 text-xs text-gray-500 group-hover:block"
              onClick={(evt) => {
                evt.stopPropagation()
                evt.preventDefault()
              }}
            >
              <CopyButton text={methodSignatureText} size={16} />
            </span>
          </div>
        )
      }
      return <HexNumber data={methodSignature} autoTruncate copyable noCopyHint className="flex-1 !font-mono" />
    },
    size: 200,
    enableResizing: true
  },
  {
    id: 'value',
    header: 'Value',
    accessorKey: 'tx.value',
    cell: (info: any) => <TransactionValue value={info.getValue() as string} chainId={info.row.original.tx.chainId} />
  },
  {
    id: 'Block Number',
    header: 'Block Number',
    accessorKey: 'blockNumber',
    size: 150,
    cell: (info: any) => <span>{info.getValue()}</span>
  },
  {
    id: 'Network',
    header: 'Network',
    accessorKey: 'tx.chainId',
    cell: (info: any) => <NetworkNode id={info.getValue()} />,
    size: 200
  },
  {
    id: 'timestamp',
    header: 'Date',
    accessorKey: 'timestamp',
    cell: (info: any) => <span>{new Date(info.getValue() * 1000).toLocaleString()}</span>,
    size: 150,
    enableResizing: false
  }
]

export { TransactionColumns }
