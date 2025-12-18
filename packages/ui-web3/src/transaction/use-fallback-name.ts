import { DecodedExternalCallTrace } from "@sentio/debugger-common"
import { useEffect, useMemo, useState } from "react"

const fallbackNameMap: Map<string, string> = new Map()
const fallbackNotifyMap: Record<string, Set<() => void>> = {}
const fallbackMapNotifiers: Array<() => void> = []

export const useFallbackName = (_address?: string, fallbackName?: string) => {
  // use timestamp to force update
  const [timestamp, setTimestamp] = useState(new Date())
  const address = _address?.toLowerCase()

  useEffect(() => {
    if (address) {
      if (!fallbackNotifyMap[address]) {
        fallbackNotifyMap[address] = new Set()
      }
      const notify = () => {
        setTimestamp(new Date())
      }
      fallbackNotifyMap[address].add(notify)
      return () => {
        fallbackNotifyMap[address].delete(notify)
      }
    }
  }, [address])

  return address ? fallbackNameMap.get(address) || '' : ''
}

export const useFallbackNameMap = () => {
  // use timestamp to force update
  const [timestamp, setTimestamp] = useState(new Date())

  useEffect(() => {
    const notify = () => {
      setTimestamp(new Date())
    }
    fallbackMapNotifiers.push(notify)
    return () => {
      fallbackMapNotifiers.splice(fallbackMapNotifiers.indexOf(notify), 1)
    }
  }, [])

  return useMemo(() => {
    // clone fallbackNameMap
    const map = new Map()
    fallbackNameMap.forEach((value, key) => {
      map.set(key, value)
    })
    return map
  }, [timestamp])
}

function setValidName(savedMap: Map<string, string>, address: string, name: string) {
  if (name && name.trim() !== "" && address && address.trim() !== "") {
    if (address.toLowerCase() !== name.toLowerCase()) {
      savedMap.set(address.toLowerCase(), name)
    }
  }
}

export const parseNamesFromTraceData = (data?: DecodedExternalCallTrace) => {
  const contractNames = new Map()
  const walk = (trace) => {
    if (trace.location) {
      if (trace.contractName) {
        if (trace.to && trace.location) {
          setValidName(contractNames, trace.to, trace.contractName)
        } else if (trace.from) {
          setValidName(contractNames, trace.from, trace.contractName)
        }
      }
      if (trace.fromContractName && trace.from) {
        setValidName(contractNames, trace.from, trace.fromContractName)
      }
      if (trace.toContractName && trace.to) {
        setValidName(contractNames, trace.to, trace.toContractName)
      }
    }
    if (trace.calls) {
      trace.calls.forEach(walk)
    }
  }
  walk(data)
  let isChanged = false
  contractNames.forEach((value, key) => {
    if (fallbackNameMap.get(key) !== value) {
      isChanged = true
      fallbackNameMap.set(key, value)
      fallbackNotifyMap[key]?.forEach((notify) => {
        notify()
      })
    }
  })
  if (isChanged) {
    fallbackMapNotifiers.forEach((notify) => notify())
  }
}