import { useMemo, useRef } from 'react'
import { useInfinite } from './use-infinite'
import { API_HOST } from './host'

export type SearchTxnRequest = {
  chainId?: string
  includeDirect?: boolean
  includeTrace?: boolean
  includeIn?: boolean
  includeOut?: boolean
  transactionStatus?: number[]
  limit?: number
  pageToken?: Uint8Array
  startBlock?: string
  endBlock?: string
  startTimestamp?: string
  endTimestamp?: string
  methodSignature?: string
}

export type SearchTxnStruct = {
  hash?: string
  blockNumber?: string
  isIn?: boolean
  trace?: boolean
  tx?: {
    blockNumber?: string
    blockHash?: string
    transactionIndex?: string
    hash?: string
    chainId?: string
    type?: string
    from?: string
    to?: string
    input?: string
    value?: string
    nonce?: string
    gas?: string
    gasPrice?: string
    maxFeePerGas?: string
    maxPriorityFeePerGas?: string
    accessList?: any[]
  }
  json?: string
  timestamp?: string
  transactionStatus?: number
  methodSignature?: string
  methodSignatureText?: string
  abiItem?: string
}

export type SearchTxnResponse = {
  transactions?: SearchTxnStruct[]
  nextPageToken?: string
}

function getUnixTime(subtractDays: number = 0) {
  return Math.floor(new Date().getTime() / 1000) - subtractDays * 3600 * 24
}

export const useSearchTxn = (req: SearchTxnRequest, limit = 10) => {
  const reqRef = useRef(req)
  const startTimestampRef = useRef<string | undefined>(undefined)
  const startTimestamp = useMemo(() => {
    if (req.startTimestamp) {
      return req.startTimestamp
    }
    if (startTimestampRef.current === undefined || JSON.stringify(reqRef.current) !== JSON.stringify(req)) {
      startTimestampRef.current = getUnixTime(7).toString()
    }
    return startTimestampRef.current
  }, [req])
  async function fetcher(req: SearchTxnRequest) {
    if (!req.chainId || !req.methodSignature) return {} as SearchTxnResponse
    const searchParams = new URLSearchParams(req as Record<string, string>)

    // set default options
    searchParams.set('includeDirect', 'true')
    searchParams.set('includeIn', 'true')
    searchParams.set('includeOut', 'true')
    searchParams.set('includeTrace', 'true')
    searchParams.set('limit', limit.toString())
    searchParams.set('startTimestamp', startTimestamp)

    const res = await fetch(`${API_HOST}/api/v1/solidity/search_transactions?${searchParams.toString()}`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json'
      }
    })
    return (await res.json()) as SearchTxnResponse
  }

  function getKey(pageIndex: number, previousPageData?: SearchTxnResponse | undefined): SearchTxnRequest | null {
    // reached the end
    if (previousPageData && (previousPageData.transactions || []).length == 0) return null
    if (pageIndex === 0) return req

    return Object.assign({}, req, { pageToken: previousPageData?.nextPageToken })
  }

  return useInfinite<SearchTxnRequest, SearchTxnResponse>(getKey, fetcher, (res) => res.transactions || [], limit, {
    revalidateFirstPage: false,
    revalidateOnFocus: false,
    keepPreviousData: true
  })
}
