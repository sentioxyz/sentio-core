import { useMemo, useContext, useEffect } from 'react'
import { Transaction, Block } from './types'
import { cx as classNames } from 'class-variance-authority'
import { chainIdToNumber, filterFundTraces, getNumberWithDecimal, isZeroValue, formatCurrency, NumberFormat } from './helpers'
import { ContractAddress } from './ContractComponents'
import { ERC20Token } from './ERC20Token'
import { getBlockTime, usePrice } from './use-price'
import multiply from 'lodash/multiply'
import sortBy from 'lodash/sortBy'
import { useAddressTag } from '../utils/use-tag'
import { CallTracesContext, ChainIdContext } from './transaction-context'
import { getNativeToken } from './ERC20Token'
import { useMobile } from '../utils/use-mobile'

interface Props {
  transaction: Transaction
  block: Block
  className?: string
  hideTitle?: boolean
  onEmpty?: (isEmpty: boolean) => void
}

type BalanceType = {
  contract: string
  balance: {
    [index: string]: {
      amount?: bigint
      price?: bigint
    }
  }
}[]

const safeToBigint = (v?: string) => {
  if (!v) return BigInt(0)
  return BigInt(v)
}

const safeAddBalance = (nodes: BalanceType, contract: string, token: string, value: string, negative?: boolean) => {
  if (!value || isZeroValue(value)) {
    return
  }
  const v = negative ? BigInt(0) - safeToBigint(value) : safeToBigint(value)
  const targetBalance = nodes.find((node) => node.contract.toLowerCase() === contract.toLowerCase())
  if (!targetBalance) {
    nodes.push({
      contract: contract?.toLowerCase(),
      balance: {
        [token]: {
          amount: BigInt(0) + v
        }
      }
    })
  } else {
    targetBalance.balance[token] = {
      amount: (targetBalance.balance[token]?.amount || BigInt(0)) + v
    }
  }
}

const numberFmt = NumberFormat({
  minimumFractionDigits: 0,
  maximumFractionDigits: 20,
  signDisplay: 'always'
})

const BalanceAmount = ({ amount, address }: { amount?: bigint; address?: string }) => {
  const chainId = useContext(ChainIdContext)
  const { data } = useAddressTag(address)
  const nativeToken = getNativeToken(chainId)
  const v =
    nativeToken.tokenAddress === address
      ? (getNumberWithDecimal(amount, nativeToken.tokenDecimals) as string)
      : (getNumberWithDecimal(amount, data?.token?.erc20?.decimals || 18) as string)
  if (!amount || amount === BigInt(0)) return <div className="text-gray font-mono">-</div>
  const isNegative = v.startsWith('-')
  return <div className={classNames('text-ilabel font-mono', !isNegative ? 'text-cyan' : 'text-red-500')}>{v}</div>
}

const BalanceValue = ({ timestamp, address, amount }: { timestamp: string | null; address?: string; amount?: bigint }) => {
  const chainId = useContext(ChainIdContext)
  const { data: tagData } = useAddressTag(address)
  const { data } = usePrice(timestamp, address, chainIdToNumber(chainId))
  if (!data || !amount || !data.price) {
    return <span className="text-gray font-mono">-</span>
  }
  const amountEther = getNumberWithDecimal(amount, tagData?.token?.erc20?.decimals || 18, true) as number
  const value = multiply(Math.abs(amountEther), data.price)
  return (
    <span className={classNames('font-mono', amountEther > BigInt(0) ? 'text-cyan' : 'text-red-500')}>
      {formatCurrency(value)}
    </span>
  )
}

export const BalanceChanges = ({ transaction, className, block, hideTitle = false, onEmpty }: Props) => {
  const { data: rootTrace, loading: txnLoading } = useContext(CallTracesContext)
  const blockTime = getBlockTime(block?.timestamp)
  const { from: sender, to: receiver, chainId } = transaction || {}
  const nativeToken = getNativeToken(chainId)
  const isMobile = useMobile()

  const changes = useMemo(() => {
    const traces = filterFundTraces(rootTrace, chainId)
    const nodes: BalanceType = []
    traces.forEach((item) => {
      if (item.address) {
        // event data
        const { events: inputs, address, name } = item
        if (name === 'Transfer') {
          const [from, to, value] = inputs
          safeAddBalance(nodes, from, address, value, true)
          safeAddBalance(nodes, to, address, value)
        } else if (name === 'Withdrawal') {
          const [from, value] = inputs
          safeAddBalance(nodes, from, address, value, true)
          safeAddBalance(nodes, address, address, value)
        } else if (name === 'Deposit') {
          const [dst, wad] = inputs
          safeAddBalance(nodes, address, address, wad, true)
          safeAddBalance(nodes, dst, address, wad)
        }
      } else {
        // calltrace data
        const { from, to, value } = item
        safeAddBalance(nodes, from, nativeToken.tokenAddress, value, true)
        safeAddBalance(nodes, to, nativeToken.tokenAddress, value)
      }
    })
    return sortBy(nodes, (node) => {
      if (node.contract === sender) {
        return 0
      }
      if (node.contract === receiver) {
        return 1
      }
      return 2
    })
  }, [rootTrace, sender, receiver, nativeToken, chainId])

  useEffect(() => {
    if (txnLoading) {
      return
    }
    onEmpty?.(changes.length === 0)
  }, [changes, txnLoading, onEmpty])

  if (changes.length === 0) {
    return null
  }

  const mobileNode = (
    <div className="overflow-x-auto">
      <table className="sm:w-full sm:min-w-max">
        <thead>
          <tr className="border-b">
            <th className="text-ilabel dark:text-text-foreground px-2 py-1 text-left font-semibold text-gray-800">
              Address
            </th>
            <th className="text-ilabel dark:text-text-foreground px-2 py-1 text-left font-semibold text-gray-800">
              Token
            </th>
            <th className="text-ilabel dark:text-text-foreground px-2 py-1 text-left font-semibold text-gray-800">
              Balance
            </th>
            <th className="text-ilabel dark:text-text-foreground px-2 py-1 text-right font-semibold text-gray-800">
              Value
            </th>
          </tr>
        </thead>
        <tbody className="divide-y">
          {changes.map((change, index) => {
            const { contract, balance } = change
            const tokens = Object.keys(balance).filter((key) => balance[key].amount)
            if (Object.values(balance).some((v) => v.amount)) {
              return tokens.map((token, tokenIndex) => {
                const amount = balance[token].amount
                return (
                  <tr key={`${contract}_${token}_${index}_${tokenIndex}`}>
                    {tokenIndex === 0 ? (
                      <td className="overflow-hidden px-2 py-2 align-top" rowSpan={tokens.length}>
                        <ContractAddress address={contract} />
                      </td>
                    ) : null}
                    <td className="overflow-hidden px-2 py-2 align-top">
                      <ERC20Token address={token} />
                    </td>
                    <td className="overflow-hidden px-2 py-2 align-top">
                      <BalanceAmount amount={amount} address={token} />
                    </td>
                    <td className="overflow-hidden px-2 py-2 text-right align-top">
                      <BalanceValue amount={amount} timestamp={blockTime} address={token} />
                    </td>
                  </tr>
                )
              })
            }

            return null
          })}
        </tbody>
      </table>
    </div>
  )

  const desktopNode = (
    <>
      <div className="grid grid-cols-5 px-2">
        <div className="text-ilabel dark:text-text-foreground col-span-2 font-semibold text-gray-800">Address</div>
        <div className="text-ilabel dark:text-text-foreground font-semibold text-gray-800">Token</div>
        <div className="text-ilabel dark:text-text-foreground font-semibold text-gray-800">Balance</div>
        <div className="text-ilabel dark:text-text-foreground text-right font-semibold text-gray-800">Value</div>
      </div>
      <div className="divide-y px-2">
        {changes.map((change, index) => {
          const { contract, balance } = change
          const tokens = Object.keys(balance).filter((key) => balance[key].amount)
          if (Object.values(balance).some((v) => v.amount)) {
            return (
              <div className="grid grid-cols-5 py-2" key={`${contract}_balanceChange_${index}`}>
                <div className="col-span-2 flex items-center overflow-hidden">
                  <ContractAddress address={contract} />
                </div>
                <div className="space-y-1 overflow-hidden">
                  {tokens.map((token, index) => (
                    <div key={`${contract}_${token}_name_${index}`}>
                      <ERC20Token address={token} />
                    </div>
                  ))}
                </div>
                <div className="space-y-2 overflow-hidden">
                  {tokens.map((token, index) => (
                    <BalanceAmount
                      key={`${contract}_${token}_amount_${index}`}
                      amount={balance[token].amount}
                      address={token}
                    />
                  ))}
                </div>
                <div className="space-y-2 overflow-hidden text-right">
                  {tokens.map((token, index) => {
                    const amount = balance[token].amount
                    return (
                      <div key={`${contract}_${token}_value_${index}`}>
                        <BalanceValue amount={amount} timestamp={blockTime} address={token} />
                      </div>
                    )
                  })}
                </div>
              </div>
            )
          }

          return null
        })}
      </div>
    </>
  )

  return (
    <div className={classNames('space-y-2', className)}>
      {!hideTitle && <div className="text-text-foreground bg-gray-50 px-2 py-2 font-semibold ">Balance Changes</div>}
      {isMobile ? mobileNode : desktopNode}
    </div>
  )
}
