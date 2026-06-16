import type { Story } from '@ladle/react'
import { useState } from 'react'
import { AggregateInput } from './AggregateInput'
import type { MetricInfoLike, QueryLike } from '../types/metrics'

const metric: MetricInfoLike = {
  contractName: ['MyToken'],
  contractAddress: ['0xabc'],
  chainId: ['1'],
  labels: {
    method: { values: ['transfer', 'approve'] },
    status: { values: ['ok', 'fail'] }
  }
}

export const Basic: Story = () => {
  const [value, setValue] = useState<QueryLike>({ query: 'erc20.transfer' })
  return (
    <div className="w-[32rem] p-8">
      <AggregateInput metric={metric} value={value} onChange={setValue} />
      <pre className="text-text-foreground-secondary mt-4 text-xs">
        {JSON.stringify(value.aggregate ?? {}, null, 2)}
      </pre>
    </div>
  )
}
Basic.meta = {
  description: 'Aggregate op selector + grouping label multi-select'
}
