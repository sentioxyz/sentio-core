import { useContext, useEffect, useState } from 'react'
import { IsSimulationContext } from '../context/transaction'
import { GlobalQueryContext } from '@sentio/ui-web3'

export const useCallTrace = (
  hash: string,
  chainId: string,
  withInternalCalls = false
) => {
  const { owner, slug } = useContext(GlobalQueryContext) as any
  const [loading, setLoading] = useState(true)
  const [data, setData] = useState<any>()
  const isSimulation = useContext(IsSimulationContext)
  useEffect(() => {
    ;(async () => {
      const data = await chrome.runtime.sendMessage({
        api: isSimulation ? 'GetCallTraceWithSimulation' : 'GetCallTrace',
        hash,
        chainId,
        withInternalCalls,
        projectOwner: owner,
        projectSlug: slug
      })
      setData(data)
      setLoading(false)
    })()
  }, [isSimulation, hash, chainId])

  return {
    data,
    loading
  }
}
