import { getChainName } from '@sentio/chain'
import type { MetricInfoLike } from '../types/metrics'

export const SystemLabels = [
  {
    field: 'contract_name',
    name: 'contract',
    getValues(metric: MetricInfoLike) {
      return (metric.contractName || []).map((name) => ({
        value: name,
        display: name
      }))
    }
  },
  {
    field: 'contract_address',
    name: 'address',
    getValues(metric: MetricInfoLike) {
      return (metric.contractAddress || []).map((name) => ({
        value: name,
        display: name
      }))
    }
  },
  {
    field: 'chain',
    name: 'chain',
    getValues(metric: MetricInfoLike) {
      return (metric.chainId || []).map((chainId) => {
        return { value: chainId, display: getChainName(chainId) }
      })
    }
  }
]

export function sortMetricByName(a: string, b: string) {
  const aIsSystem = a.startsWith('system.')
  const bIsSystem = b.startsWith('system.')

  if (aIsSystem && !bIsSystem) {
    return 1
  }
  if (!aIsSystem && bIsSystem) {
    return -1
  }
  return a.localeCompare(b)
}
