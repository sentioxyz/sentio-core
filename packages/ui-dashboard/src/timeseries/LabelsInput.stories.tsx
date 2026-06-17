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
    status: { values: ['ok', 'fail'] },
    from: { values: ['0xabc123', '0xdef456', '0xghi789'] },
    to: { values: ['0x987654', '0x543210', '0xabcdef'] },
    token: { values: ['ETH', 'DAI', 'USDC'] },
    event: { values: ['swap', 'stake', 'withdraw'] },
    network: { values: ['mainnet', 'polygon', 'optimism'] },
    role: { values: ['sender', 'receiver', 'operator'] },
    action: { values: ['create', 'update', 'delete'] },
    source: { values: ['web', 'api', 'bot'] }
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
