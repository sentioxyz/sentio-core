import { useMemo, useCallback, useEffect } from 'react'
import { useResizeDetector } from 'react-resize-detector'
import { useTransactions } from '~/content/lib/debug/use-transaction'
import { sentioTxUrl } from '~/utils/url'
import { ResizeTable } from '@sentio/ui-core'
import {
  BaseDialog,
  BarLoading,
  CopyButton,
  TransactionColumns,
  chainIdToNumber
} from '@sentio/ui-web3'

interface Props {
  open: boolean
  onClose: () => void
  chainId: string
  address?: string
  sig?: string
}

export const RelatedTxn = ({ chainId, open, onClose, sig, address }: Props) => {
  const txQuery = useMemo(() => {
    if (!sig || !chainId || !address) {
      return
    }
    const query = {
      chainId: [chainId],
      address: [address],
      includeDirect: true,
      includeIn: true,
      includeOut: true,
      includeTrace: true,
      methodSignature: sig
    }

    return query
  }, [sig, address, chainId])

  const toTxn = useCallback((hash, chainId) => {
    window.open(sentioTxUrl(chainId, hash), '_blank')
  }, [])

  const { width, height, ref } = useResizeDetector()
  const columns = useMemo(() => {
    if (!width) {
      return TransactionColumns.filter((col) => col.id !== 'Network')
    }

    let leftWidth = width
    const leftColumns: any[] = []
    const columns = TransactionColumns.filter(
      (col) => col.id !== 'Network'
    ).map((col) => {
      const clonedCol = Object.assign({}, col)
      if (col.size) {
        leftWidth -= col.size
      } else {
        leftColumns.push(clonedCol)
      }
      return clonedCol
    })
    leftWidth -= 20
    leftColumns.forEach((col) => {
      // minimum column width is 100
      col.size = Math.max(leftWidth / leftColumns.length, 100)
    })
    return columns
  }, [width])

  const { transactions, fetchNextPage, hasMore, isRefreshing, loading } =
    useTransactions(txQuery) || {}
  const tableHeight = `${Math.max(400, height ? height : 0)}px`

  return (
    <BaseDialog
      open={open}
      onClose={onClose}
      title="Related Transactions"
      panelClassName="md:max-w-2xl lg:max-w-4xl xl:max-w-5xl 2xl:max-w-7xl"
      footer={
        <div className="text-icontent px-4 py-1">
          <span className="space-x-1">
            <span className="text-gray-800">Contract Address:</span>
            <span className="text-primary">{address}</span>
            <span className="inline-block align-text-bottom">
              <CopyButton text={address} size={16} />
            </span>
          </span>
          <span className="ml-4 space-x-1">
            <span className="text-gray-800">Function Signature:</span>
            <span className="text-primary">{sig}</span>
            <span className="inline-block align-text-bottom">
              <CopyButton text={sig} size={16} />
            </span>
          </span>
        </div>
      }
    >
      <div className="min-h-[60vh] px-4" ref={ref}>
        {transactions?.length > 0 ? (
          <ResizeTable
            columns={columns}
            data={transactions}
            columnResizeMode="onChange"
            allowEditColumn={false}
            allowSort={false}
            onFetchMore={fetchNextPage}
            hasMore={hasMore}
            onClick={(row) => {
              const { hash, tx } = row.original
              toTxn(hash, chainIdToNumber(tx?.chainId))
            }}
            height={tableHeight}
            isFetching={isRefreshing}
          />
        ) : loading ? (
          <div
            style={{
              height: tableHeight
            }}
          >
            <BarLoading hint="Loading Transactions" />
          </div>
        ) : (
          <div className="h-fit">
            <div className="mt-6 text-center">
              <h1 className="text-lg font-medium">
                No matching transactions found
              </h1>
            </div>
          </div>
        )}
      </div>
    </BaseDialog>
  )
}
