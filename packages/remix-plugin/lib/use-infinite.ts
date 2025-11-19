import { useCallback } from 'react'
import useSWRInfinite, { SWRInfiniteConfiguration } from 'swr/infinite'

type GetKey<RESP, REQ> = (pageIndex: number, previousResponse?: RESP) => REQ | null

type Fetcher<RESP> = (REQ: any) => Promise<RESP>

type GetItems<RESP> = (response: RESP) => any[]

export function useInfinite<REQ, RESP>(
  getKey: GetKey<RESP, REQ>,
  fetcher: Fetcher<RESP>,
  getItems: GetItems<RESP>,
  pageSize: number,
  options?: SWRInfiniteConfiguration
) {
  // TODO yulong check if we can remove as any
  const { data, error, isValidating, mutate, size, setSize } = useSWRInfinite(getKey as any, fetcher, options)

  const isLoadingInitialData = !data && !error
  const isLoadingMore = isLoadingInitialData || (size > 0 && data && typeof data[size - 1] === 'undefined')
  const isEmpty = (data && data[0] ? getItems(data[0]) : []).length === 0
  const isReachingEnd = isEmpty || (data && (getItems(data[data.length - 1]) || []).length < pageSize)

  const isRefreshing = isValidating && data && data.length === size
  const fetchNextPage = useCallback(() => {
    setSize((size) => size + 1)
  }, [setSize])

  return {
    data,
    isLoadingMore,
    isReachingEnd,
    isRefreshing,
    isEmpty,
    fetchNextPage,
    error,
    mutate
  }
}
