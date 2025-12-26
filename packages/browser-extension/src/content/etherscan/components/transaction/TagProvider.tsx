import { filterAddressFromCallTrace, TagCacheContext } from '@sentio/ui-web3'
import { useContext, useEffect, useRef } from 'react'
import { sortBy } from 'lodash'

interface Props {
  callTrace?: any
  chain?: string
}

export const TagProvider = ({ callTrace, chain }: Props) => {
  const { setTagCache, clearTagCache } = useContext(TagCacheContext)
  const addressSetRef = useRef<Set<string>>(new Set())

  useEffect(() => {
    const addressList = sortBy(filterAddressFromCallTrace(callTrace))
    if (addressList.length === 0) {
      return
    }
    const addressSet = addressSetRef.current
    const newAddressSet = new Set<string>()
    addressList.forEach((addr) => {
      const lowerAddr = addr.toLowerCase()
      if (addressSet.has(lowerAddr)) {
        return
      }
      newAddressSet.add(lowerAddr)
      return addressSet.add(lowerAddr)
    })
    if (newAddressSet.size === 0) {
      return
    }
    async function fetchNewTags() {
      const data = await chrome.runtime.sendMessage({
        api: 'MultiGetTagByAddress',
        requests: Array.from(newAddressSet).map((address) => ({
          chainId: chain,
          address
        }))
      })
      const nameMap = new Map()
      data.responses.forEach((d) => {
        if (d.address && d.primaryName) {
          nameMap.set(d.address, d)
        }
      })
      setTagCache(nameMap)
    }
    fetchNewTags()
  }, [callTrace, chain])

  useEffect(() => {
    // clear tag cache
    return () => {
      clearTagCache()
    }
  }, [])
  return null
}
