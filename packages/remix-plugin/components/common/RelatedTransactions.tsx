import { Fragment } from 'react'
import { cn } from '@/lib/utils'
import { useSearchTxn, SearchTxnRequest, SearchTxnStruct } from '@/lib/use-search-txn'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableFooter, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { TransactionStatus } from './TxnStatus'
import { PropagateLoader } from 'react-spinners'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import localizeFormat from 'dayjs/plugin/localizedFormat'
dayjs.extend(relativeTime)
dayjs.extend(localizeFormat)
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { BigNumber } from 'bignumber.js'
import { ChainDecimals } from './networks'
import { API_HOST } from '@/lib/host'

const BN = BigNumber.clone({
  EXPONENTIAL_AT: [-50, 50]
})

interface Props {
  open?: boolean
  chainId?: SearchTxnRequest['chainId']
  methodSignature?: SearchTxnRequest['methodSignature']
}

function trimTxnHash(str?: string) {
  if (!str) {
    return str
  }
  return str.slice(0, 10) + '...' + str.slice(-4)
}

function trimHex(str?: string) {
  if (!str) {
    return str
  }
  return str.slice(0, 6) + '...' + str.slice(-4)
}

function getTxnLink(chainId: string, hash: string) {
  const network = chainId.startsWith('0x') ? chainId.replace('0x', '') : chainId
  return `${API_HOST}/tx/${network}/${hash}`
}

function parseValue(rawValue?: string, _chainId?: string) {
  if (rawValue === undefined || _chainId === undefined) {
    return rawValue
  }
  const chainId = _chainId.startsWith('0x') ? _chainId.replace('0x', '') : ''
  const { decimal, unit } = ChainDecimals[chainId] || ChainDecimals['1']
  const value = BN(rawValue).div(new BN(10).pow(decimal)).toFormat()
  return `${value} ${unit}`
}

export const RelatedTransactions = ({ open, chainId, methodSignature }: Props) => {
  const { data, isLoadingMore, isReachingEnd, mutate, fetchNextPage } = useSearchTxn(
    {
      chainId,
      methodSignature
    },
    60
  )
  return (
    <div className={cn(open ? '' : 'hidden', 'mt-2 overflow-auto rounded-lg border p-1')}>
      <Table className="block h-fit max-h-[50vh] overflow-auto">
        <TableHeader className="bg-light sticky top-0 shadow-sm">
          <TableRow>
            <TableHead className="w-[160px] whitespace-nowrap">Txn Hash</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>From</TableHead>
            <TableHead>To</TableHead>
            <TableHead>Value</TableHead>
            <TableHead>Block</TableHead>
            <TableHead>Time</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoadingMore && !data
            ? new Array(10).fill(0).map((item, index) => (
                <TableRow className="animate-pulse" key={index}>
                  <TableCell className="p-2">
                    <div className="h-[21px] w-32 rounded-md bg-gray-300 dark:bg-gray-500" />
                  </TableCell>
                  <TableCell className="p-2">
                    <div className="h-[21px] w-20 rounded-md bg-gray-300 dark:bg-gray-500" />
                  </TableCell>
                  <TableCell className="p-2">
                    <div className="h-[21px] w-32 rounded-md bg-gray-300 dark:bg-gray-500" />
                  </TableCell>
                  <TableCell className="p-2">
                    <div className="h-[21px] w-32 rounded-md bg-gray-300 dark:bg-gray-500" />
                  </TableCell>
                  <TableCell className="p-2">
                    <div className="h-[21px] w-20 rounded-md bg-gray-300 dark:bg-gray-500" />
                  </TableCell>
                  <TableCell className="p-2">
                    <div className="h-[21px] w-20 rounded-md bg-gray-300 dark:bg-gray-500" />
                  </TableCell>
                  <TableCell className="p-2">
                    <div className="h-[21px] w-32 rounded-md bg-gray-300 dark:bg-gray-500" />
                  </TableCell>
                </TableRow>
              ))
            : null}
          {data?.map((txnList, index) => {
            return (
              <Fragment key={index}>
                {txnList?.transactions?.map((txn: SearchTxnStruct, index: number) => (
                  <TableRow key={index} className="text-xs">
                    <TableCell className="font-mono">
                      <a href={getTxnLink(txn.tx!.chainId!, txn.hash!)} target="_blank" className="hover:underline">
                        {trimTxnHash(txn.hash)}
                      </a>
                    </TableCell>
                    <TableCell>
                      <TransactionStatus status={txn.transactionStatus} />
                    </TableCell>
                    <TableCell className="font-mono">{trimHex(txn.tx?.from)}</TableCell>
                    <TableCell className="font-mono">{trimHex(txn.tx?.to)}</TableCell>
                    <TableCell className="whitespace-nowrap">{parseValue(txn.tx?.value, txn.tx?.chainId!)}</TableCell>
                    <TableCell>{txn.blockNumber}</TableCell>
                    <TableCell>
                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span className="whitespace-nowrap">
                              {txn.timestamp ? dayjs.unix(parseInt(txn.timestamp, 10)).fromNow() : ''}
                            </span>
                          </TooltipTrigger>
                          <TooltipContent>
                            <span>{txn.timestamp ? dayjs.unix(parseInt(txn.timestamp, 10)).format('LLL') : ''}</span>
                          </TooltipContent>
                        </Tooltip>
                      </TooltipProvider>
                    </TableCell>
                  </TableRow>
                ))}
              </Fragment>
            )
          })}
        </TableBody>
        <TableFooter>
          <TableRow>
            <TableCell colSpan={7}>
              {isLoadingMore ? (
                !data ? null : (
                  <div className="sticky bottom-0 left-0 flex w-full items-center justify-center py-6">
                    <PropagateLoader color="#0756d577" size={12} />
                  </div>
                )
              ) : isReachingEnd ? (
                <div className="text-info sticky bottom-0 left-0 text-center">No more transactions</div>
              ) : (
                <div className="sticky bottom-0 left-0 flex w-full justify-center">
                  <Button
                    size="sm"
                    onClick={() => {
                      fetchNextPage()
                    }}
                  >
                    Load more
                  </Button>
                </div>
              )}
            </TableCell>
          </TableRow>
        </TableFooter>
      </Table>
    </div>
  )
}
