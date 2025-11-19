import { ContractItemType, SignatureMapItem } from '@/lib/use-global-store'
import { useCallback, useState } from 'react'
import { ContractSelect } from './ContractSelect'
import { CompiledContract, RemixTxEvent } from '@remixproject/plugin-api'
import { RecentTransactions } from './RecentTransactions'
import { FunctionItem, CompileSpecType } from '@/lib/types'

interface Props {
  data?: ContractItemType[]
  txnData?: RemixTxEvent[]
  functions?: FunctionItem[]
  getMethodBySelector?: (selector: string) => SignatureMapItem[] | undefined
  beforeSimulate?: (targetPath: string, targetContract: string) => CompileSpecType
}

export const TransactionTab = ({ data = [], txnData = [], functions, getMethodBySelector, beforeSimulate }: Props) => {
  const [abiData, setAbiData] = useState<CompiledContract['abi']>([])
  const onSelectContract = useCallback((v: ContractItemType) => {
    setAbiData(v.data.abi)
  }, [])
  return (
    <>
      {/* <div className="px-2">
        <ContractSelect data={data} onSelect={onSelectContract} />
      </div> */}
      <RecentTransactions
        data={txnData}
        functions={functions}
        getMethodBySelector={getMethodBySelector}
        beforeSimulate={beforeSimulate}
      />
    </>
  )
}
