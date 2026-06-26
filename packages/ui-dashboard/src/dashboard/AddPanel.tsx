import { forwardRef, useCallback, useState } from 'react'
import { Button } from '@sentio/ui-core'
import { LuPlus } from 'react-icons/lu'
import { AddPanelSlideover } from './AddPanelSlideover'
import type { ChartLike, ChartTypeLike, DataSourceTypeLike } from '../types'

export const defaultMetricChart: ChartLike = {
  type: 'LINE',
  datasourceType: 'METRICS'
}
export const defaultNoteChart: ChartLike = {
  type: 'NOTE',
  note: {
    content: ''
  },
  datasourceType: 'NOTES'
}
export const defaultAnalyticChart: ChartLike = {
  type: 'LINE',
  datasourceType: 'ANALYTICS',
  config: {
    timeRangeOverride: {
      timeRange: {
        interval: { unit: 'd', value: 1 }
      }
    }
  }
}
export const defaultInsightChart: ChartLike = {
  type: 'LINE',
  datasourceType: 'INSIGHTS',
  insightsQueries: [
    {
      dataSource: 'METRICS',
      metricsQuery: {
        query: '',
        alias: '',
        id: 'a'
      }
    }
  ],
  config: {
    timeRangeOverride: {
      timeRange: {}
    }
  }
}
export const defaultEventChart: ChartLike = {
  type: 'TABLE',
  datasourceType: 'EVENTS',
  eventLogsConfig: {
    columnsConfig: {},
    query: '',
    timeRangeOverride: {
      enabled: false
    }
  }
}
export const defaultGroupChart: ChartLike = {
  type: 'GROUP',
  datasourceType: 'GROUP',
  group: {
    title: 'New Group',
    collapsed: false
  }
}
export const defaultRetentionChart: ChartLike = {
  type: 'LINE',
  datasourceType: 'RETENTION',
  config: {
    timeRangeOverride: {
      timeRange: {}
    }
  },
  retentionQuery: {
    resources: [
      { eventNames: [], filter: { timeFilter: { type: 'Disable' } } },
      { eventNames: [], filter: { timeFilter: { type: 'Disable' } } }
    ],
    windowSize: 7,
    interval: { unit: 'Day', value: 1 },
    groupBy: [],
    segmentBy: [],
    criteria: 'On'
  }
}
export const defaultSqlChart: ChartLike = {
  type: 'TABLE',
  datasourceType: 'SQL',
  sqlQuery: JSON.stringify({ sql: '', size: 100, version: 'AUTO' })
}

type RouterQuery = { [key: string]: string | string[] | undefined }

interface Props {
  allowEdit: boolean
  saving: boolean
  onNewPanel: (chart: ChartLike) => void
  onImportPanel: () => void
  /** Current route params used to build SQL/insights "new panel" links. Injected by the consumer. */
  routerQuery?: RouterQuery
  generateLinkHref?: (
    chartType: ChartTypeLike,
    datasource: DataSourceTypeLike,
    query: RouterQuery
  ) => string
}

export const AddPanel = forwardRef(function AddPanel(
  { onNewPanel, onImportPanel, saving, routerQuery, generateLinkHref }: Props,
  ref: any
) {
  const [slideOverVisible, setSlideOverVisible] = useState(false)
  const closeSlideOver = useCallback(() => setSlideOverVisible(false), [])
  const openSlideOver = useCallback(() => setSlideOverVisible(true), [])

  const onSelectNewPanel = useCallback(
    (type: string) => {
      const [chartCategory, chartType] = type.split('.')
      let chart: ChartLike | undefined

      switch (chartCategory) {
        case 'import':
          onImportPanel()
          closeSlideOver()
          return
        case 'annotations':
          chart = { ...defaultNoteChart }
          break
        case 'group':
          closeSlideOver()
          onNewPanel({ ...defaultGroupChart })
          return
        case 'analytics':
          chart = { ...defaultAnalyticChart }
          break
        case 'insights':
          chart = { ...defaultInsightChart }
          break
        case 'events':
          chart = { ...defaultEventChart }
          break
        case 'retention':
          chart = { ...defaultRetentionChart }
          break
        case 'sql':
          chart = { ...defaultSqlChart }
          break
        case 'timeseries':
        default:
          chart = { ...defaultMetricChart }
      }

      switch (chartType) {
        case 'line':
          chart.type = 'LINE'
          break
        case 'bar':
          chart.type = 'BAR'
          break
        case 'area':
          chart.type = 'AREA'
          break
        case 'bargauge':
          chart.type = 'BAR_GAUGE'
          break
        case 'queryvalue':
          chart.type = 'QUERY_VALUE'
          break
        case 'table':
          chart.type = 'TABLE'
          break
        case 'pie':
          chart.type = 'PIE'
          break
        case 'note':
          chart.type = 'NOTE'
          break
        case 'log':
          chart.type = 'TABLE'
          break
        default:
          chart = undefined
      }

      if (chart) {
        closeSlideOver()
        onNewPanel(chart)
      }
    },
    [closeSlideOver, onNewPanel, onImportPanel]
  )

  return (
    <div className="inline-flex">
      <Button
        ref={ref}
        processing={saving}
        icon={<LuPlus />}
        role="primary"
        onClick={openSlideOver}
        size="md"
      >
        Add Panel
      </Button>
      <AddPanelSlideover
        open={slideOverVisible}
        onClose={closeSlideOver}
        onSelect={onSelectNewPanel}
        routerQuery={routerQuery}
        generateLinkHref={generateLinkHref}
        allowImport
      />
    </div>
  )
})
