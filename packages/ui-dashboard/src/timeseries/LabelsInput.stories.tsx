import type { Story } from '@ladle/react'
import { useState } from 'react'
import { LabelsInput } from './LabelsInput'
import type { MetricInfoLike, QueryLike } from '../types/metrics'

const metric: MetricInfoLike = {
  contractName: ['MyToken'],
  contractAddress: ['0xabc123'],
  chainId: ['1', '137'],
  labels: {
    method: { values: ['transfer', 'approve', 'mint'] },
    status: { values: ['ok', 'fail'] }
  }
}

export const Basic: Story = () => {
  const [value, setValue] = useState<QueryLike>({ query: 'erc20.transfer' })
  return (
    <div className="w-[32rem] p-8">
      <LabelsInput metric={metric} value={value} onChange={setValue} />
      <pre className="text-text-foreground-secondary mt-4 text-xs">
        {JSON.stringify(value.labelSelector ?? {}, null, 2)}
      </pre>
    </div>
  )
}
Basic.meta = {
  description: 'Label selector multi-select populated from a metric'
}
