import { useContext } from "react"
import { TagsContext } from "@sentio/ui-core"
import Web3 from 'web3'
const web3 = new Web3()

export const chainIdToNumber = (chainId?: string) => {
  if (!chainId) return undefined
  if (chainId.includes('_')) {
    // not a EVM chain
    return chainId
  }
  if (chainId.startsWith('0x')) {
    return parseInt(chainId, 16)
  }
  return parseInt(chainId)
}

function upperFirst(str: string): string {
  if (!str) return ''
  return str.charAt(0).toUpperCase() + str.slice(1)
}

export function getPathHostName(link?: string) {
  if (!link) {
    return ''
  }
  try {
    const url = new URL(link)
    return upperFirst(url?.host?.replace('.com', '').replace('.io', '') || '')
  } catch {
    return ''
  }
}

export const useAddressTag = (address?: string) => {
  const addressMap = useContext(TagsContext)
  const lowerAddress = address?.toLowerCase()
  return {
    data: lowerAddress ? addressMap.get(lowerAddress) : undefined
  }
}

export function toChecksumAddress(address: string) {
  if (typeof address !== 'string' || !address) {
    return address
  }

  address = address.toLowerCase().replace('0x', '')
  const sha3 = web3.utils.sha3(address)
  if (!sha3) {
    return address
  }
  const hash = sha3.replace('0x', '')
  let checksumAddress = '0x'

  for (let i = 0; i < address.length; i++) {
    if (parseInt(hash[i], 16) >= 8) {
      checksumAddress += address[i].toUpperCase()
    } else {
      checksumAddress += address[i]
    }
  }

  return checksumAddress
}