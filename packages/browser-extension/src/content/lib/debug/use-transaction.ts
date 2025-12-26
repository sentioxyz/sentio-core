import {
  EvmSearchTransactionsRequest,
  EvmSearchTransactionsResponse,
  EvmRawTransaction
} from '~/content/lib/types/evm-search'
import { EthChainId } from '@sentio/chain'
import { useMemo } from 'react'
import { useInfinite } from './use-infinite'

const PAGE_SIZE = 50

export const useTransactions = (
  searchQuery?: Partial<EvmSearchTransactionsRequest>
) => {
  const getKey = (
    pageIndex: number,
    previousPageData?: EvmSearchTransactionsResponse
  ) => {
    // if (!projectId) return null

    // reached the end
    if (previousPageData && (previousPageData.transactions || []).length == 0)
      return null

    // if (!projectId) {
    //   return null
    // }
    const req: Partial<EvmSearchTransactionsRequest> = {
      address: []
    }
    if (searchQuery) {
      Object.assign(req, searchQuery)
    }
    if (req?.address?.length === 0 || !req) {
      return null
    }

    // add default options
    req.limit = req.limit || PAGE_SIZE
    req.chainId = req.chainId || [EthChainId.ETHEREUM]
    req.includeIn = true
    req.includeOut = true
    req.pageToken = previousPageData?.nextPageToken

    return req
  }

  const fetcher = async (req) => {
    return await chrome.runtime.sendMessage({
      api: 'SearchTransaction',
      requests: req
    })
  }

  const {
    data,
    error,
    isLoadingMore,
    mutate,
    isReachingEnd,
    fetchNextPage,
    isRefreshing,
    isEmpty
  } = useInfinite(getKey, fetcher, (res) => res.transactions || [], PAGE_SIZE, {
    revalidateFirstPage: false,
    revalidateOnFocus: false
  })

  const transactions = useMemo(() => {
    return data?.reduce((acc, cur) => {
      return acc.concat(cur.transactions || [])
    }, [] as EvmRawTransaction[])
  }, [data])

  return {
    error,
    loading: !data && !error,
    mutate,
    transactions,
    fetchNextPage,
    hasMore: !isReachingEnd,
    isLoadingMore,
    isRefreshing,
    isEmpty
  }
}
