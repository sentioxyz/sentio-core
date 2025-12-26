import { useEffect, useState, useContext } from 'react'
import { GlobalQueryContext } from '@sentio/ui-web3'

export const useSimulator = (simulationId?: string) => {
  const { owner, slug } = useContext(GlobalQueryContext) as any
  const [loading, setLoading] = useState(true)
  const [data, setData] = useState<any>()
  useEffect(() => {
    setLoading(true)
    if (!simulationId) {
      setData(undefined)
      return
    }
    ;(async () => {
      const req = {
        simulationId
      }
      if (owner) {
        req['projectOwner'] = owner
      }
      if (slug) {
        req['projectSlug'] = slug
      }
      const data = await chrome.runtime.sendMessage({
        api: 'GetSimulation',
        data: req
      })
      setData(data)
      setLoading(false)
    })()
  }, [owner, slug, simulationId])

  return {
    data,
    loading
  }
}
