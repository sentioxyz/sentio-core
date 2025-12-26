import { isNumber, isObject, isSafeInteger } from 'lodash'
import { Source, ExtendedCall, parseMonacoUriFn } from './types'
import pick from 'lodash/pick'
import cloneDeep from 'lodash/cloneDeep'
import { BD, parseHex, getNumberWithDecimal } from '@sentio/ui-core'
import Web3 from 'web3'
import { AbiItem } from 'web3-utils'
import { DecodedCallTrace, DecodedLog } from '@sentio/debugger-common'
import isNumeric from 'validator/lib/isNumeric'
import { pickBy, upperFirst } from 'lodash'
import { Monaco } from '@monaco-editor/react'
import { safeNativize as _safeNativize } from '@sentio/debugger-common'

export { parseHex, getNumberWithDecimal }

export const web3: Web3 = new Web3()

export function NumberFormat(options: Intl.NumberFormatOptions) {
  return new Intl.NumberFormat('en-US', options)
}

const regexExternal =
  /externalFor\((0x[0-9a-fA-F]+)\)Via\(([a-z]+)\)Number\((\d+)\)/
const regexUserSource = /userSourceFor\((0x[0-9a-fA-F]+)\)Number\((\d+)\)/
export const contractAddressRegex = /^0x[a-fA-F0-9]{40}$/
export function parseCompilationId(raw?: string) {
  if (!raw) {
    return
  }

  let results = regexExternal.exec(raw)
  if (results) {
    return {
      address: results[1],
      source: results[2],
      number: results[3]
    }
  }
  results = regexUserSource.exec(raw)
  if (results) {
    return {
      address: results[1],
      source: '',
      number: results[2]
    }
  }
}
export function getCompilationId(
  address: string,
  source = 'etherscan',
  number = '0'
) {
  return `externalFor(${address})Via(${source})Number(${number})`
}

export function parseFileName(raw?: string) {
  if (!raw) {
    return ''
  }
  const index = raw.lastIndexOf('/')
  if (index === -1) {
    return raw.replace('.sol', '')
  }
  return raw.substring(index + 1).replace('.sol', '')
}

export function getValueString(data: any): string {
  if (data === undefined || data === null) {
    return ''
  }
  const isArrayType = Array.isArray(data)
  if (isArrayType) {
    return data.map((item: any) => getValueString(item)).join(', ')
  }
  const isObjectType = isObject(data)
  if (isObjectType) {
    return JSON.stringify(data) ?? '...'
  }
  if (isNumber(data) && !isSafeInteger(data)) {
    return BigInt(data).toString()
  }

  return data.toString() ?? '...'
}

// export const solFetcher = async (params: Record<string, string>) => {
//   const {
//     hash,
//     networkId,
//     txIdentifierKey = 'txId.txHash',
//     chainIdentifierKey = 'chainSpec.chainId',
//     ...otherParams
//   } = params

//   if (!hash || !networkId) {
//     return
//   }

//   const adminMode = getStorageValue('sentio_admin_mode')
//   const reqParams = pickBy(
//     {
//       [chainIdentifierKey]: networkId,
//       [txIdentifierKey]: hash,
//       ...otherParams
//     },
//     (value) => value !== undefined
//   )
//   const req = `/api/v1/solidity/fetch_and_compile?${new URLSearchParams(reqParams)}`
//   const headers = new Headers()
//   if (otherParams.projectOwner && otherParams.projectSlug) {
//     const token = await getAccessToken()
//     headers.set('Content-Type', 'application/json')
//     if (token && token !== 'anonymous') {
//       headers.set('Authorization', `Bearer ${token}`)
//       if (adminMode) {
//         headers.set('x-admin-mode', 'true')
//       }
//     }
//   }

//   const res = await fetch(req, {
//     headers
//   })
//   if (!res.ok) {
//     throw new Error('failed to fetch')
//   }
//   return (res as any).json()
// }

export function getSourcePathKey(data: Source) {
  if (data.address) {
    return `file:///${data.address}/${data.filePath}`
  }
  const { compilationId } = data
  const { address } = parseCompilationId(compilationId) || {}
  return `file:///${address}/${data.filePath}`
}

export function isZeroValue(data: string) {
  return data === '0x0' || data === '0x' || data === '0'
}

export function isBurnAddress(address?: string) {
  return address ? parseInt(address, 16) === 0 : false
}

export const numberFmt = NumberFormat({
  minimumFractionDigits: 0,
  maximumFractionDigits: 20
})

export function displayNumber(
  hex?: string | bigint,
  unit?: string,
  target?: 'ether' | 'gwei',
  hideUnit?: boolean
) {
  if (!hex) {
    return null
  }
  const bigInt = typeof hex === 'bigint' ? hex : parseHex(hex)
  if (unit === 'wei' && target === 'gwei') {
    return getNumberWithDecimal(bigInt, 9) + (!hideUnit ? ' Gwei' : '')
  } else if (unit === 'wei' && target === 'ether') {
    return getNumberWithDecimal(bigInt, 18) + (!hideUnit ? ' Ether' : '')
  }
  return bigInt.toLocaleString() + (unit ? ` ${unit}` : '')
}

/**
 * Get hex string by multiple 10^decimal
 * @param hex init value
 * @param decimal 10^decimal
 * @returns hex string
 */
export function getHexStringByMultiple(
  hex?: string | bigint,
  decimal?: number
) {
  if (hex === undefined || decimal === undefined) {
    return null
  }
  const bdInstance = typeof hex === 'bigint' ? BD(hex.toString()) : BD(hex)
  const n = bdInstance.multipliedBy(BD(10).pow(decimal))
  return `0x${n.toString(16)}`
}

export function filterTraceEvents(callTraces: any[]) {
  const events: any[] = []
  const walk = (entry: any) => {
    const { logs, calls } = entry
    if (logs?.length > 0) {
      events.push(...(logs as any[]))
    }
    calls?.forEach(walk)
  }
  callTraces?.forEach(walk)
  return events
}

const ABI: AbiItem[] = [
  {
    anonymous: false,
    inputs: [
      {
        indexed: true,
        name: 'from',
        type: 'address'
      },
      {
        indexed: true,
        name: 'to',
        type: 'address'
      },
      {
        indexed: false,
        name: 'value',
        type: 'uint256' // TODO if
      }
    ],
    name: 'Transfer',
    type: 'event'
  },
  {
    anonymous: false,
    inputs: [
      {
        indexed: true,
        name: 'src',
        type: 'address'
      },
      {
        indexed: false,
        name: 'wad',
        type: 'uint256'
      }
    ],
    name: 'Withdrawal',
    type: 'event'
  },
  {
    anonymous: false,
    inputs: [
      {
        indexed: true,
        name: 'dst',
        type: 'address'
      },
      {
        indexed: false,
        name: 'wad',
        type: 'uint256'
      }
    ],
    name: 'Deposit',
    type: 'event'
  }
]

const EVENT_MAP = new Map<string, number>()

for (const [idx, abiItem] of ABI.entries()) {
  EVENT_MAP.set(web3.eth.abi.encodeEventSignature(abiItem as any), idx)
}

export function decodeLog(log: any) {
  const idx = EVENT_MAP.get(log?.topics?.[0])
  if (idx === undefined) {
    return undefined
  }
  const abiItem = ABI[idx]
  if (!abiItem.inputs) {
    return undefined
  }

  // if (idx > 0 && !isWrappedNativeToken(log.address)) {
  //   return undefined
  // }
  try {
    const event = web3.eth.abi.decodeLog(
      abiItem.inputs,
      log.data,
      log.topics.slice(1)
    )
    const arr = []
    for (let i = 0; i < abiItem.inputs.length; i++) {
      // @ts-ignore actually has index
      arr.push(event[i])
    }
    return {
      ...log,
      name: abiItem.name!,
      events: arr
    }
  } catch (e) {
    // ignore
    console.error(e)
  }

  return undefined
}

export function filterFundTraces(
  rootTrace: DecodedCallTrace,
  chainId?: string
) {
  const res: any[] = []
  const walk = (entry: any) => {
    // TODO add typing
    const { logs, calls, value, error, type } = entry

    if (error) {
      return
    }
    // DELEGATECALL value should always be 0, API issue
    if (type !== 'DELEGATECALL' && value && !isZeroValue(value)) {
      res.push(pick(entry, ['from', 'to', 'value', 'startIndex']))
    }

    logs.forEach((rawLog: any) => {
      const log = decodeLog(rawLog)
      if (!log) {
        return
      }
      try {
        if (log.name === 'Transfer') {
          const [from, to, value] = log.events
          if (isNumeric(value)) {
            res.push(log)
          }
        } else if (log.name === 'Withdrawal') {
          const [from, value] = log.events
          if (isNumeric(value)) {
            res.push(log)
          }
        } else if (log.name === 'Deposit') {
          const [dst, wad] = log.events
          if (isNumeric(wad)) {
            res.push(log)
          }
        }
      } catch {
        // ignore
      }
    })
    calls?.forEach(walk)
  }
  if (rootTrace) {
    walk(rootTrace)
  }
  /**
   * Filter out duplicated ERC20 transfer events of deposit and withdrawal
   * eg. tx/534352/0x40749674fe7a47715927a978f50d282dbcbe33dab3bc9ad432131b70413dc8cb
   */
  const filteredRes: any[] = []
  for (let i = 0; i < res.length; i++) {
    const log = res[i]
    const nextLog = res[i + 1]
    if (nextLog) {
      if (log.name === 'Transfer' && nextLog.name === 'Deposit') {
        const [from, to, value] = log.events
        const [dst, wad] = nextLog.events
        if (wad === value && to === dst && isBurnAddress(from)) {
          //This is a duplicate transfer event of deposit
          continue
        }
      } else if (log.name === 'Transfer' && nextLog.name === 'Withdrawal') {
        const [from, to, value] = log.events
        const [src, wad] = nextLog.events
        if (wad === value && from === src && isBurnAddress(to)) {
          //This is a duplicate transfer event of withdrawal
          continue
        }
      }
    }
    filteredRes.push(log)
  }
  return filteredRes
}

export function trimFilePath(rawPath?: string) {
  if (!rawPath) {
    return ''
  }
  // trim last file name
  const index = rawPath.lastIndexOf('/')
  if (index === -1) {
    return rawPath
  }
  return rawPath.substring(index + 1)
}

// internal call's op includes jump
export function isInternalCall(call: ExtendedCall) {
  const { type } = call
  return (type as string)?.toLowerCase().includes('jump')
}

export function isStaticCall(call: ExtendedCall) {
  const { type } = call
  return (type as string)?.toLowerCase() === 'staticcall'
}

function filterTraces(
  root: ExtendedCall,
  filterFn: (call: ExtendedCall) => boolean
) {
  const walk = (call: any, parentPath: any[]) => {
    const { calls = [], logs = [], ...extra } = call
    if (filterFn(call)) {
      const parent = parentPath[parentPath.length - 1]
      if (parent) {
        parent.logs = parent.logs || []
        parent.logs.push(...logs)
      }
      calls.forEach((c: any) => walk(c, parentPath))
      return undefined
    } else {
      const newNode = { ...extra, logs: logs }
      const parent = parentPath[parentPath.length - 1]
      if (parent) {
        parent.calls = parent.calls || []
        parent.calls.push(newNode)
      }
      calls.forEach((c: any) => walk(c, [...parentPath, newNode]))
      return newNode
    }
  }
  const newRoot = walk(cloneDeep(root), [])
  const rebuild = (call: any, depth = 0) => {
    // const { calls = [] } = call
    // return {
    //   ...call,
    //   depth,
    //   calls: calls.map((c) => rebuild(c, depth + 1)),
    // }
    call.depth = depth
    call.calls?.map((c: any) => rebuild(c, depth + 1))
    return call
  }
  return rebuild(newRoot)
}

export function getExternalTrace(root: ExtendedCall) {
  return filterTraces(root, isInternalCall)
}

export function filterStaticTrace(root: ExtendedCall) {
  return filterTraces(root, isStaticCall)
}

export function filterInternalAndStaticTrace(root: ExtendedCall) {
  return filterTraces(
    root,
    (call) => isInternalCall(call) || isStaticCall(call)
  )
}

export function getExternalTraceMove(root: ExtendedCall) {
  return filterTraces(root, (call) => call.depth > 1 && call.from === call.to)
}

export function findTransactionError(root: DecodedCallTrace) {
  let resError = ''
  const walk = (call: any) => {
    const { calls = [], error, revertReason } = call
    if (error) {
      resError = error
    }
    if (revertReason) {
      resError += ` (${revertReason})`
    }
    if (!resError) {
      calls.forEach(walk)
    }
  }
  walk(root)
  return resError
}

export function isAddressType(type?: string) {
  if (!type) {
    return false
  }
  return type === 'address' || type.startsWith('contract')
}

export function isArrayType(type?: string) {
  return type?.includes('[]') || type === 'tuple' // fix API type issue
}

export function setCallTraceKeys(root?: ExtendedCall) {
  const walk = (
    call: any,
    currentPrefix: any,
    parentError = false,
    parentFunctionName: any = undefined
  ) => {
    const { calls = [], logs = [], error, storages = [] } = call
    ;(call as any).wkey = currentPrefix.toString()
    ;(call as any).parentFunctionName = parentFunctionName
    Object.assign(call, parentError ? { parentError: true } : {})
    calls?.forEach((item: any, index: number) => {
      walk(
        item,
        `${currentPrefix}.c${index}`,
        !!error || parentError,
        call.functionName
      )
    })
    logs?.forEach((item: any, index: number) => {
      item.wkey = `${currentPrefix}.e${index}`
      item.parentError = !!error || parentError
    })
    storages?.forEach((item: any, index: number) => {
      item.wkey = `${currentPrefix}.s${index}`
      item.contractName = call.contractName
    })
  }
  if (root) {
    walk(root, 0, false, undefined)
  }
  // root?.forEach((call, index) => {
  //   walk(call, `${index}`)
  // })
  return root
}

export function setCallTraceParentFunction(root?: ExtendedCall) {
  const walk = (
    call: any,
    currentPrefix: any,
    parentError = false,
    parentFunctionName: any = undefined
  ) => {
    const { calls = [], logs = [], error } = call
    ;(call as any).parentFunctionName = parentFunctionName
    Object.assign(call, parentError ? { parentError: true } : {})
    calls?.forEach((item: any, index: number) => {
      walk(
        item,
        `${currentPrefix}.c${index}`,
        !!error || parentError,
        call.functionName
      )
    })
    logs?.forEach((item: any, index: number) => {
      item.parentError = !!error || parentError
    })
  }
  if (root) {
    walk(root, 0, false, undefined)
  }
  return root
}

export const chainIdToNumber = (chainId?: string) => {
  if (!chainId) return undefined
  if (chainId.startsWith('0x')) {
    return parseInt(chainId, 16)
  }
  return parseInt(chainId)
}

export const filterAddressFromCallTrace = (data?: DecodedCallTrace) => {
  const addressSet = new Set<string>()
  const walkLog = (logData: DecodedLog) => {
    const { address, events } = logData
    addressSet.add(address)
    events?.forEach((event) => {
      if (event.type === 'address') {
        addressSet.add(event.value)
      }
    })
  }
  const walk = (call: DecodedCallTrace) => {
    const { from, address, calls = [], logs = [], inputs, returnValue } = call
    if (from) {
      addressSet.add(from)
    }
    if (address) {
      addressSet.add(address)
    }
    inputs?.forEach((input: any) => {
      if (input.type === 'address') {
        addressSet.add(input.value)
      }
    })
    if (returnValue) {
      if (Array.isArray(returnValue)) {
        returnValue.forEach((val) => {
          if (val.type === 'address') {
            addressSet.add(val.value)
          }
        })
      } else {
        if (returnValue.type === 'address') {
          addressSet.add(returnValue.value)
        }
      }
    }
    calls.forEach(walk)
    logs.forEach(walkLog)
  }
  if (data) {
    walk(data)
  }
  return Array.from(addressSet).filter((addr) => addr !== undefined)
}

export const parseUri: parseMonacoUriFn = (uri) => {
  if (!uri) {
    return {
      address: '',
      path: ''
    }
  }
  const pathList = uri.path.split('/')
  const address = pathList[1]
  const path = pathList.slice(2).join('/')
  return {
    address,
    path
  }
}

export const safeCreateModel = (
  monaco: Monaco,
  source: any,
  fileUri: Parameters<Monaco['editor']['getModel']>[0]
) => {
  let newModel = monaco.editor.getModel(fileUri)
  if (!newModel) {
    try {
      newModel = monaco.editor.createModel(source, 'sentio-solidity', fileUri)
    } catch (e) {
      console.error(e)
    }
  }
  return newModel
}

const nf = NumberFormat({
  style: 'currency',
  currency: 'USD',
  maximumFractionDigits: 18
})

const nf2 = NumberFormat({
  style: 'currency',
  currency: 'USD',
  maximumFractionDigits: 2
})

export const formatCurrency = (value: number, maxValidDigits = 2) => {
  if (value < 0.01) {
    const res = nf.format(value)
    const [integer, decimal] = res.split('.')
    const firstValidDigitIndex = decimal
      ?.split('')
      .findIndex((digit) => digit !== '0')
    if (firstValidDigitIndex === -1) {
      return res
    } else {
      const validDecimal = decimal?.substring(
        0,
        firstValidDigitIndex + maxValidDigits
      )
      return `${integer}.${validDecimal}`
    }
  }
  return nf2.format(value)
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

export function getLastRevertReason(rootTrace?: DecodedCallTrace) {
  if (
    !rootTrace ||
    !rootTrace.error ||
    rootTrace.error.includes('revert') === false
  ) {
    return undefined
  }

  let res: any = undefined
  const walk = (call: DecodedCallTrace) => {
    const { error, calls } = call
    const decodedError = (call as any).decodedError
    if (error && error.includes('revert') && decodedError) {
      res = decodedError
    }
    calls?.forEach(walk)
  }

  walk(rootTrace)
  return res
}

export function trimAddress(address?: string) {
  if (!address) {
    return ''
  }
  return address.substring(0, 6) + '...' + address.substring(address.length - 4)
}

export function trimAptosAddress(address?: string) {
  if (!address) {
    return ''
  }
  let _address = address
  if (address.startsWith('0x')) {
    _address = address.substring(2)
  }
  if (
    _address ===
    '0000000000000000000000000000000000000000000000000000000000000001'
  ) {
    return '0x1'
  } else if (
    _address ===
    '0000000000000000000000000000000000000000000000000000000000000002'
  ) {
    return '0x2'
  }
  return '0x' + trimAddress(_address)
}

export function safeNativize(value: any) {
  try {
    return _safeNativize(value)
  } catch (e) {
    console.error('Failed to nativize value, reason:', e)
    return undefined
  }
}
