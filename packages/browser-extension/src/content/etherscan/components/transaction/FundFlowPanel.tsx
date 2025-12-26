import { useContext, useEffect, useState } from 'react'
import { useSetAtom } from 'jotai'
import { EthChainId } from '@sentio/chain'
import {
  TransactionFundflow,
  SpinLoading,
  SvgFolderContext,
  filterAddressFromCallTrace,
  TagCacheContext
} from '@sentio/ui-web3'

interface Props {
  hash: string
  chainId: string
  allCallTraces?: any
  transaction?: any
  transactionLoading?: boolean
  callTrace?: any
  callTraceLoading?: boolean
}

export const FundFlowPanel = ({
  hash,
  chainId,
  allCallTraces,
  transaction,
  transactionLoading,
  callTrace,
  callTraceLoading
}: Props) => {
  const { setTagCache } = useContext(TagCacheContext)
  const [tagLoading, setTagLoading] = useState(chainId !== EthChainId.ETHEREUM)
  const [isEmpty, setIsEmpty] = useState(false)

  useEffect(() => {
    if (!allCallTraces || chainId === EthChainId.ETHEREUM) {
      return
    }
    const addresses = filterAddressFromCallTrace(allCallTraces)
    if (addresses.length) {
      ;(async () => {
        const data = await chrome.runtime.sendMessage({
          api: 'MultiGetTagByAddress',
          requests: addresses.map((address) => ({ chainId, address }))
        })
        const nameMap = new Map()
        data.responses.forEach((d) => {
          if (d.address && d.primaryName) {
            nameMap.set(d.address, d)
          }
        })
        setTagCache(nameMap)
        setTagLoading(false)
      })()
    } else {
      setTagLoading(false)
    }
  }, [chainId, allCallTraces])

  return (
    <SvgFolderContext.Provider value="https://app.sentio.xyz/">
      <SpinLoading
        loading={callTraceLoading || transactionLoading || tagLoading}
        showMask
        className={isEmpty ? '' : 'min-h-[240px]'}
      >
        {callTrace && transaction && !tagLoading && (
          <TransactionFundflow
            data={callTrace}
            transaction={transaction.transaction}
            empty={
              <div className="relative h-full py-4 text-center">
                <span className="text-gray-400">
                  This transaction has no fund flow
                </span>
              </div>
            }
            onEmpty={setIsEmpty}
            chainId={chainId}
          />
        )}
      </SpinLoading>
    </SvgFolderContext.Provider>
  )
}
