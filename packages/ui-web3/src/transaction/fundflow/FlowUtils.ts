import { DecodedCallTrace } from '@sentio/debugger-common'
import { filterFundTraces, getNumberWithDecimal } from '../helpers'
import sortBy from 'lodash/sortBy'
import uniq from 'lodash/uniq'
import { getChainExternalUrl } from '@sentio/chain'
import { getNativeToken } from '../ERC20Token'

const TokenColors = [
  '#f05a4d',
  '#56bce5',
  '#73ba46',
  '#ff9f05',
  '#ad56e2',
  '#e97ec2',
  '#2e71db'
]
const LabelInfoSuffix = 'ⁱ'

export type TransferItem = {
  from: string
  to: string
  value: string
  tokenAddress?: string
  tokenName?: string
  chainId: string
}

function formatCompactNumber(value: string): string {
  const num = parseFloat(value)

  if (isNaN(num)) {
    return value
  }

  // Handle very small numbers (keep 3 significant digits)
  if (num !== 0 && Math.abs(num) < 1) {
    const formatted = num.toPrecision(3)
    return parseFloat(formatted).toString()
  }

  // Handle large numbers with suffixes
  const abs = Math.abs(num)

  if (abs >= 1e9) {
    return (num / 1e9).toPrecision(3).replace(/\.?0+$/, '') + 'B'
  } else if (abs >= 1e6) {
    return (num / 1e6).toPrecision(3).replace(/\.?0+$/, '') + 'M'
  } else if (abs >= 1e3) {
    return (num / 1e3).toPrecision(3).replace(/\.?0+$/, '') + 'K'
  }

  // For numbers >= 1 and < 1000, keep 3 significant digits
  return num.toPrecision(3).replace(/\.?0+$/, '')
}

// Helper to convert chainId string to number
function chainIdToNumber(chainId: string): number {
  return parseInt(chainId)
}

export function getNode(
  address: string,
  sender?: string,
  receiver?: string,
  addressName?: string,
  link?: string,
  trimAddress?: boolean
) {
  let tag = ''
  if (sender?.toLowerCase() === address?.toLowerCase()) {
    tag = '[Sender]'
  } else if (receiver?.toLowerCase() === address?.toLowerCase()) {
    tag = '[Receiver]'
  }
  let labelName = addressName
  labelName = labelName?.replaceAll('\u0000', '')
  if (!labelName) {
    labelName = address.toLowerCase()
    labelName = trimAddress
      ? labelName.substring(0, 6) +
        '...' +
        labelName.substring(labelName.length - 4)
      : labelName
  }
  return `"${address.toLowerCase()}" [label="${trimAddress ? tag || labelName : `${labelName}${tag ? ' ' + tag : ''}`}" class="hover:underline underline-offset-2" URL="${link}" target="_blank" color="${
    tag ? '#7EA7E9' : '#CDDDF7'
  }"]`
}

export function getEdge(
  index: number,
  from: string,
  to: string,
  value: string,
  tokenAddress?: string,
  tokenName?: string,
  color = '#eee',
  link?: string,
  decimals = 18,
  theme = 'light',
  trimAmount = false
) {
  const tokenValue = value.startsWith('0x') ? value : BigInt(value)
  let name = ''
  if (tokenName) {
    name = tokenName
  } else if (tokenAddress) {
    name = `(Token ${tokenAddress.substring(0, 4)}...${tokenAddress.substring(tokenAddress.length - 4)})`
  }

  const displayLink = `"${from?.toLowerCase()}" -> "${to?.toLowerCase()}"`
  let displayValue = getNumberWithDecimal(tokenValue, decimals)

  // Apply compact formatting if trimAmount is enabled
  if (trimAmount && displayValue) {
    displayValue = formatCompactNumber(String(displayValue))
  }

  const labelName =
    theme === 'light'
      ? `${displayValue} ${name}`
      : `<font COLOR="#E4E4E4">${displayValue} ${name}</font>`

  const labelString = `<<table href="${link}" target="_blank" border="0" title="[${index}] ${displayValue} ${name}"><tr><td><font COLOR="${color}">[${index}]</font></td><td>${labelName}</td></tr></table>>`

  return [
    displayLink,
    ` [label=${labelString} color="${color}" id="${name}" class="flow-chart-edge"]`
  ].join('')
}

export function safeAddNode(targetNode: any, nodes: any[]) {
  const target = nodes.find((n) => targetNode === n)
  if (!target) {
    nodes.push(targetNode)
  }
}

export function processDecodedCallTrace(
  data: DecodedCallTrace,
  chainId: string,
  setTagAddressList?: (addresses: string[]) => void
): TransferItem[] {
  const nativeToken = getNativeToken(chainId)
  let items: TransferItem[] = []
  const fundItems = sortBy(filterFundTraces(data, chainId), 'startIndex')
  const transferList: TransferItem[] = []
  const tokenAddresses = new Set<string>()
  const nodeAddresses = new Set<string>()

  fundItems.forEach((data) => {
    if (data.address) {
      // events
      const { events: inputs, address, name } = data
      switch (name) {
        case 'Transfer': {
          const [from, to, value] = inputs
          if (value === '0') {
            return
          }
          transferList.push({
            from: from,
            to: to,
            value: value,
            tokenAddress: address,
            chainId
          })
          tokenAddresses.add(from)
          tokenAddresses.add(to)
          tokenAddresses.add(address)
          nodeAddresses.add(from)
          nodeAddresses.add(to)
          break
        }
        case 'Withdrawal': {
          const [from2, value2] = inputs
          if (value2 === '0') {
            return
          }
          transferList.push({
            from: from2,
            to: address,
            value: value2,
            tokenAddress: address,
            chainId
          })
          tokenAddresses.add(from2)
          tokenAddresses.add(address)
          nodeAddresses.add(from2)
          break
        }
        case 'Deposit': {
          const [dst, wad] = inputs
          if (wad === '0') {
            return
          }
          transferList.push({
            from: address,
            to: dst,
            value: wad,
            tokenAddress: address,
            chainId
          })
          tokenAddresses.add(dst)
          tokenAddresses.add(address)
          nodeAddresses.add(dst)
          break
        }
      }
    } else {
      // call trace
      const { from, to, value } = data
      if (value === '0' || !value) {
        return
      }
      transferList.push({
        from,
        to,
        value,
        chainId,
        tokenAddress: nativeToken.tokenAddress,
        tokenName: nativeToken.tokenSymbol
      })
      tokenAddresses.add(from)
      tokenAddresses.add(to)
      nodeAddresses.add(from)
      nodeAddresses.add(to)
    }
  })

  if (transferList.length > nodeAddresses.size * 2) {
    // merge transferList by from to and tokenAddress
    const transferMap = new Map<string, TransferItem>()
    transferList.forEach((item) => {
      const key = `${item.from}_${item.to}_${item.tokenAddress}`
      if (transferMap.has(key)) {
        const pre = transferMap.get(key)
        if (pre) {
          pre.value = (BigInt(pre.value) + BigInt(item.value)).toString()
        }
      } else {
        transferMap.set(key, item)
      }
    })
    items = Array.from(transferMap.values())
  } else {
    items = transferList
  }

  if (items.length === 0) {
    return []
  }

  const addressList = sortBy(
    uniq(Array.from(tokenAddresses).map((address) => address.toLowerCase()))
  )
  setTagAddressList?.(addressList)

  return items
}

export function generateNodesAndEdges({
  items,
  transactionFrom,
  transactionTo,
  chainId,
  tagMap,
  defaultTagMap,
  isDarkMode,
  trimAddress,
  trimAmount
}: {
  items: TransferItem[]
  transactionFrom: string
  transactionTo: string
  chainId: string
  tagMap: Map<string, any>
  defaultTagMap: Map<string, string>
  isDarkMode: boolean
  trimAddress?: boolean
  trimAmount?: boolean
}) {
  const cid = chainIdToNumber(chainId)
  const nodes: any[] = []
  const edges: any[] = []
  const colorMap = new Map<string, string>()
  let colorIndex = 0
  let index = 1

  items.forEach((item) => {
    const { from, to, value, tokenAddress, tokenName } = item
    let erc20TokenName = tokenName
    let tokenDecimals = 18
    if (!erc20TokenName && tokenAddress) {
      const lowerTokenAddress = tokenAddress.toLowerCase()
      erc20TokenName = tagMap
        .get(lowerTokenAddress)
        ?.token?.erc20?.symbol?.toUpperCase()
      tokenDecimals =
        tagMap.get(lowerTokenAddress)?.token?.erc20?.decimals || 18
    }
    let fromName = tagMap.get(from.toLowerCase())?.primaryName
    if (!fromName) {
      fromName = defaultTagMap.get(from.toLowerCase())
      if (fromName) {
        fromName += LabelInfoSuffix
      }
    }
    let toName = tagMap.get(to.toLowerCase())?.primaryName
    if (!toName) {
      toName = defaultTagMap.get(to.toLowerCase())
      if (toName) {
        toName += LabelInfoSuffix
      }
    }
    safeAddNode(
      getNode(
        from,
        transactionFrom,
        transactionTo,
        fromName,
        getChainExternalUrl(cid, from, 'address'),
        trimAddress
      ),
      nodes
    )
    safeAddNode(
      getNode(
        to,
        transactionFrom,
        transactionTo,
        toName,
        getChainExternalUrl(cid, to, 'address'),
        trimAddress
      ),
      nodes
    )
    if (!tokenAddress) {
      return
    }
    let color: string | undefined
    if (colorMap.has(tokenAddress)) {
      color = colorMap.get(tokenAddress)
    } else {
      color = TokenColors[colorIndex % TokenColors.length]
      colorIndex++
      colorMap.set(tokenAddress, color)
    }
    edges.push(
      getEdge(
        index++,
        from,
        to,
        value,
        tokenAddress,
        erc20TokenName,
        color,
        getChainExternalUrl(cid, tokenAddress, 'address'),
        tokenDecimals,
        isDarkMode ? 'dark' : 'light',
        trimAmount
      )
    )
  })

  const sortedNodes = sortBy(nodes, (n: any) => {
    if (n.id === transactionFrom) {
      return 0
    }
    if (n.id === transactionTo) {
      return 1
    }
    return 2
  })

  return { nodes: sortedNodes, edges }
}
