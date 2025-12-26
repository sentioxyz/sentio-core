import { useState } from 'react'
import { BalanceChanges, SpinLoading } from '@sentio/ui-web3'

interface Props {
  transaction?: any
  data?: any
  loading?: boolean
}

type CoinIDType =
  | {
      symbol: string
    }
  | {
      address: {
        address: string
        chain: string
      }
    }

export type GetPriceRequest = {
  timestamp: string
  coinId: CoinIDType
}

export const BalanceChangePanel = ({ transaction, data, loading }: Props) => {
  const [isEmpty, setEmpty] = useState(false)
  return (
    <SpinLoading
      loading={loading}
      showMask
      className={loading ? 'min-h-[200px] pb-4' : 'pb-4'}
    >
      {isEmpty ? (
        <div className="text-center text-gray-400">No balance changes</div>
      ) : null}
      <BalanceChanges
        transaction={transaction?.transaction}
        block={transaction?.block}
        hideTitle
        onEmpty={setEmpty}
        data={data}
        loading={loading}
      />
    </SpinLoading>
  )
}
