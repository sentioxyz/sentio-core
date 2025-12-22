import { useMemo } from 'react'
import { cx as classNames } from 'class-variance-authority'
import { MevLink } from './MevLink'

export interface SandwichTx {
  txHash?: string
  txIndex?: number
}

export interface SandwichResult {
  txs?: SandwichTx[]
  victims?: SandwichTx[]
}

interface SandwichTxnsProps {
  data: SandwichResult
  currentTxHash: string
  chainId?: string
}

export const SandwichTxns = ({
  data,
  currentTxHash,
  chainId
}: SandwichTxnsProps) => {
  const { txs, victims } = data
  const allTxns = useMemo(() => {
    const txns = [
      ...(txs || []).map((item) => {
        return {
          ...item,
          type: 'attacker'
        }
      }),
      ...(victims || []).map((item) => ({ ...item, type: 'victim' }))
    ]
    return txns.sort((a, b) => a.txIndex! - b.txIndex!)
  }, [txs, victims])
  return (
    <div className="w-full">
      {allTxns.map((item, index) => (
        <div className={classNames('py-1')} key={index}>
          <div
            className={classNames(
              'grid w-full grid-cols-3 gap-2 rounded-md px-2 py-0.5',
              item.type === 'attacker' ? 'bg-red-100' : 'bg-gray-100'
            )}
          >
            <span className="w-fit font-medium capitalize">
              {item.type}
              {item.txHash === currentTxHash ? ' (current)' : ''}
            </span>
            <span className="text-center">
              <MevLink
                data={item.txHash}
                type="tx"
                truncate
                chainId={chainId}
              />
            </span>
            <span className="min-w-20 whitespace-nowrap text-right">
              Position: {item.txIndex}
            </span>
          </div>
        </div>
      ))}
    </div>
  )
}
