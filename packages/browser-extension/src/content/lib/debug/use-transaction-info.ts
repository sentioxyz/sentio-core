import { useEffect, useState, useContext } from 'react'
import { IsSimulationContext } from '../context/transaction'

export const useTransactionInfo = (
  hash: string,
  chainId: string,
  withStateDiff = false
) => {
  const [loading, setLoading] = useState(true)
  const [data, setData] = useState<any>()
  const isSimulation = useContext(IsSimulationContext)
  useEffect(() => {
    ;(async () => {
      const data = await chrome.runtime.sendMessage({
        api: isSimulation
          ? 'GetTransactionInfoWithSimulation'
          : 'GetTransactionInfo',
        hash,
        chainId,
        withStateDiff
      })
      setData(data)
      setLoading(false)
    })()
  }, [isSimulation])

  return {
    data,
    loading
  }
}

export async function getBlockIndexByHash(hash: string, chainId: string) {
  try {
    const data = await chrome.runtime.sendMessage({
      api: 'GetTransactionInfo',
      hash,
      chainId
    })
    return Number.parseInt(data.transaction.transactionIndex, 16)
  } catch {
    return -1
  }
}

export async function getTransactions(txHashList: string[], networkId: string) {
  try {
    if (!txHashList || txHashList.length === 0 || !networkId) {
      return {}
    }
    const data = await chrome.runtime.sendMessage({
      api: 'GetTransactions',
      txHashList,
      networkId
    })
    return data?.transactions || {}
  } catch {
    return {}
  }
}
