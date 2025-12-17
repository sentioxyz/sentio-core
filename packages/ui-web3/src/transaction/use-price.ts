import useSWRImmutable from 'swr/immutable'
import dayjs from 'dayjs'
import { getNativeToken } from './ERC20Token'
import { useContext } from 'react'
import { PriceFetcherContext } from './transaction-context'

export function getBlockTime(hex: string) {
  if (!hex) {
    return null
  }
  const timestamp = BigInt(hex) * BigInt(1000)
  const time = dayjs(Number(timestamp))
  return time.toDate().toISOString()
}

export const usePrice = (timestamp?: any, address?: string, chain?: string | number) => {
  const priceFetcher = useContext(PriceFetcherContext)
  let req: any = {
    timestamp,
    coinId: {
      address: {
        address,
        chain
      }
    }
  }
  if (!address || !timestamp || chain === undefined || isNaN(chain as any)) {
    req = undefined
  } else {
    const nativeToken = getNativeToken(chain?.toString())
    if (address === nativeToken.tokenAddress) {
      req.coinId.address.address = nativeToken.priceTokenAddress
    }
  }

  const { data, error } = useSWRImmutable(req, priceFetcher, {
    shouldRetryOnError: false,
    onError: () => {
      //ignore error
    }
  })
  return {
    data,
    isLoading: !error && !data,
    isError: error
  }
}
