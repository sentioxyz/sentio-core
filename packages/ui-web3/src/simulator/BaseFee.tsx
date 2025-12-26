import { memo, useRef } from 'react'
import { useFormContext } from 'react-hook-form'
import { useAtomValue } from 'jotai/react'
import { simulationFormState } from './atoms'
import { getWeiAmount } from './AmountUnitSelect'
import { BigDecimal } from '@sentio/bigdecimal'
import dayjs from 'dayjs'

const BD = BigDecimal.clone({
  EXPONENTIAL_AT: [-30, 30]
})

const formatCurrency = (value: number) => {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD'
  }).format(value)
}

const getNativeToken = (networkId?: string) => {
  // Default to ETH
  return {
    tokenSymbol: 'ETH',
    priceTokenAddress: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
    chainId: '1'
  }
}

const BaseFee = () => {
  const { watch } = useFormContext()
  const value = watch('value')
  const gasLimit = watch('gas') as string
  const gasPrice = watch('gasPrice') as string
  const networkId = watch('contract.chainId') as string
  const atomFormState = useAtomValue(simulationFormState)

  let baseFee = BD('0')
  try {
    if (value) {
      baseFee = baseFee.plus(getWeiAmount(value, atomFormState.valueUnit))
    }
    if (gasLimit && gasPrice) {
      baseFee = baseFee.plus(
        BD(gasLimit).multipliedBy(
          getWeiAmount(gasPrice, atomFormState.gasPriceUnit)
        )
      )
    }
    baseFee = baseFee.div(BD(10).pow(18))
  } catch {
    // do nothing
  }
  const nativeToken = getNativeToken(networkId)

  return (
    <div className="text-ilabel flex gap-2">
      <span>Value + Transaction Fee (Max Amount)</span>
      <span className="text-primary-800/70">=</span>
      <span className="text-primary-800/70 font-mono">
        {baseFee.toString()} {nativeToken.tokenSymbol}
      </span>
    </div>
  )
}

export default memo(BaseFee)
