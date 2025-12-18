import BigDecimal from '@sentio/bigdecimal'
export const BD = BigDecimal.clone({
  EXPONENTIAL_AT: 1e9
})

export function parseHex(hex: string = '0'): bigint {
  try {
    return BigInt(BD(hex).toString())
  } catch {
    return BigInt(0)
  }
}

export function getNumberWithDecimal(hex?: string | bigint, decimal?: number, asNumber?: boolean) {
  if (hex === undefined || decimal === undefined) {
    return null
  }
  const bigInt = typeof hex === 'bigint' ? hex : parseHex(hex)
  const n = BD(bigInt.toString()).div(decimal > 0 ? BD(10).pow(decimal) : 1)
  if (asNumber) {
    return n.toNumber()
  }
  return n.toString()
}