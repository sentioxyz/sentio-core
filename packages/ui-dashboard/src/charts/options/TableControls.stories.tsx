import type { Story } from '@ladle/react'
import { useState } from 'react'
import { TableControls } from './TableControls'
import type { TableConfigLike, TableDataLike } from '../../types'

// Metrics-style result: columns are derived per series via getColumnNameId.
const metricsData: TableDataLike = {
  results: [
    {
      alias: '{{method}}',
      matrix: {
        samples: [
          {
            metric: { labels: { method: 'transfer' }, displayName: 'Transfers' }
          },
          {
            metric: { labels: { method: 'approve' }, displayName: 'Approvals' }
          }
        ]
      }
    }
  ]
}

// SQL-style result: columns come straight from columnTypes.
const sqlData: TableDataLike = {
  result: {
    columnTypes: { block_number: 'NUMBER', tx_hash: 'STRING', ts: 'TIME' }
  }
}

export const Metrics: Story = () => {
  const [config, setConfig] = useState<TableConfigLike>({})
  return (
    <div className="w-[36rem] p-8">
      <TableControls
        config={config}
        defaultOpen
        onChange={setConfig}
        data={metricsData}
      />
    </div>
  )
}

export const Sql: Story = () => {
  const [config, setConfig] = useState<TableConfigLike>({})
  return (
    <div className="w-[36rem] p-8">
      <TableControls
        config={config}
        defaultOpen
        onChange={setConfig}
        data={sqlData}
      />
    </div>
  )
}
