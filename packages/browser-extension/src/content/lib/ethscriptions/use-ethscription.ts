import { useState, useEffect } from 'react'

export const useEthScription = (id?: string) => {
  const [loading, setLoading] = useState(true)
  const [data, setData] = useState<any>()
  useEffect(() => {
    ;(async () => {
      if (!id) {
        return
      }
      const data = await chrome.runtime.sendMessage({
        api: 'GetInscription',
        id
      })
      setData(data)
      setLoading(false)
    })()
  }, [id])

  return {
    data,
    loading
  }
}
