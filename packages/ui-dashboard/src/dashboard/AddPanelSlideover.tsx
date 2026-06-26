import { useCallback } from 'react'
import {
  SlideOver,
  AreasIcon,
  BarsIcon,
  GaugeIcon,
  GroupsIcon,
  LinesIcon,
  NoteIcon,
  PieIcon,
  QueryValueIcon,
  TableIcon,
  SQlIcon,
  EventLogsIcon,
  ScatterIcon,
  ImportIcon
} from '@sentio/ui-core'
import type { ChartTypeLike, DataSourceTypeLike } from '../types'

type RouterQuery = { [key: string]: string | string[] | undefined }

const PanelCard = ({
  icon,
  label,
  onClick,
  dataType
}: {
  icon: React.ReactNode
  label: string
  onClick?: (evt: React.MouseEvent) => void
  dataType: string
}) => {
  return (
    <div
      onClick={(e) => onClick?.(e)}
      className="group cursor-pointer space-y-2"
      data-type={dataType}
    >
      <div className="group-hover:bg-hover rounded-md p-4 transition-colors">
        <div className="mx-auto grid h-14 w-14 items-center justify-center">
          {icon}
        </div>
      </div>
      <div className="group-hover:text-text-foreground text-text-foreground-secondary w-full text-center text-xs group-hover:font-medium">
        {label}
      </div>
    </div>
  )
}

const AnotationsPanels = [
  {
    icon: <NoteIcon />,
    label: 'Note',
    type: 'annotations.note'
  }
]

const InsightsPanels: {
  icon: React.ReactNode
  label: string
  type: string
  chartType: ChartTypeLike
}[] = [
  {
    icon: <LinesIcon />,
    label: 'Lines',
    type: 'insights.line',
    chartType: 'LINE'
  },
  { icon: <BarsIcon />, label: 'Bars', type: 'insights.bar', chartType: 'BAR' },
  {
    icon: <AreasIcon />,
    label: 'Areas',
    type: 'insights.area',
    chartType: 'AREA'
  },
  {
    icon: <GaugeIcon />,
    label: 'Bar Gauge',
    type: 'insights.bargauge',
    chartType: 'BAR_GAUGE'
  },
  {
    icon: <ScatterIcon />,
    label: 'Scatter',
    type: 'insights.scatter',
    chartType: 'SCATTER'
  },
  {
    icon: <QueryValueIcon />,
    label: 'Query Value',
    type: 'insights.queryvalue',
    chartType: 'QUERY_VALUE'
  },
  {
    icon: <TableIcon />,
    label: 'Table',
    type: 'insights.table',
    chartType: 'TABLE'
  },
  { icon: <PieIcon />, label: 'Pie', type: 'insights.pie', chartType: 'PIE' }
]

const EventsPanels = [
  {
    icon: <EventLogsIcon />,
    label: 'Event Logs',
    type: 'events.log'
  }
]

const SqlPanels = [
  {
    icon: <SQlIcon />,
    label: 'SQL Chart',
    type: 'sql.table',
    chartType: 'TABLE' as ChartTypeLike
  }
]

const ContainerPanels = [
  {
    icon: <GroupsIcon />,
    label: 'Empty Group',
    type: 'group.container'
  }
]

interface Props extends React.ComponentProps<typeof SlideOver> {
  onSelect?: (type: string) => void
  /** Current route params used to build "new panel" links. Injected by the consumer. */
  routerQuery?: RouterQuery
  generateLinkHref?: (
    chartType: ChartTypeLike,
    datasource: DataSourceTypeLike,
    query: RouterQuery
  ) => string
  allowImport?: boolean
}

function defaultLinkGenerator(
  chartType: ChartTypeLike,
  datasource: DataSourceTypeLike,
  query: RouterQuery
) {
  const { owner, slug, id: dashboardId, ...restQuery } = query
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(restQuery)) {
    if (Array.isArray(value)) {
      value.forEach((v) => search.append(key, v))
    } else if (value != null) {
      search.append(key, value)
    }
  }
  const qs = search.toString()
  const url = `/${owner}/${slug}/dashboards/${dashboardId}/panels/new`
  return `${url}${qs ? `?${qs}` : ''}#tab=edit&chart_type=${chartType}&datasource=${datasource}`
}

export const AddPanelSlideover = ({
  open,
  onClose,
  onSelect,
  routerQuery = {},
  generateLinkHref = defaultLinkGenerator,
  allowImport
}: Props) => {
  const onClick = useCallback(
    (evt: React.MouseEvent) => {
      evt.stopPropagation()
      const target = evt.currentTarget as HTMLElement
      const type = target.getAttribute('data-type')
      if (type && onSelect) {
        onSelect(type)
      }
    },
    [onSelect]
  )

  return (
    <SlideOver open={open} onClose={onClose} title="Add Panel" size="xs">
      <div className="relative h-full w-full space-y-6 overflow-y-auto px-4 py-6">
        {allowImport && (
          <div>
            <h3 className="text-ilabel font-ilabel text-text-foreground-secondary mb-2">
              Import & SQL
            </h3>
            <div className="grid grid-cols-2 gap-x-2.5 gap-y-5">
              <PanelCard
                icon={<ImportIcon className="h-14 w-auto" />}
                dataType=""
                label="Import"
                onClick={(evt: React.MouseEvent) => {
                  evt.stopPropagation()
                  onSelect?.('import')
                }}
              />
              {SqlPanels.map(({ icon, label, type }, index) => (
                <a
                  key={index}
                  href={generateLinkHref('TABLE', 'SQL', routerQuery)}
                >
                  <PanelCard icon={icon} label={label} dataType={type} />
                </a>
              ))}
            </div>
          </div>
        )}

        <div>
          <h3 className="text-ilabel font-ilabel text-text-foreground-secondary mb-2">
            Containers
          </h3>
          <div className="grid grid-cols-2 gap-x-2.5 gap-y-5">
            {ContainerPanels.map(({ icon, label, type }, index) => (
              <PanelCard
                key={index}
                icon={icon}
                label={label}
                dataType={type}
                onClick={onClick}
              />
            ))}
          </div>
        </div>

        <div>
          <h3 className="text-ilabel font-ilabel text-text-foreground-secondary mb-2">
            Insights
          </h3>
          <div className="grid grid-cols-2 gap-x-2.5 gap-y-5">
            {InsightsPanels.map(({ icon, label, type, chartType }, index) => (
              <a
                key={index}
                href={generateLinkHref(chartType, 'INSIGHTS', routerQuery)}
              >
                <PanelCard icon={icon} label={label} dataType={type} />
              </a>
            ))}
          </div>
        </div>

        <div>
          <h3 className="text-ilabel font-ilabel text-text-foreground-secondary mb-2">
            Anotations
          </h3>
          <div className="grid grid-cols-2 gap-x-2.5 gap-y-5">
            {AnotationsPanels.map(({ icon, label, type }, index) => (
              <PanelCard
                key={index}
                icon={icon}
                label={label}
                dataType={type}
                onClick={onClick}
              />
            ))}
          </div>
        </div>

        <div>
          <h3 className="text-ilabel font-ilabel text-text-foreground-secondary mb-2">
            Events
          </h3>
          <div className="grid grid-cols-2 gap-x-2.5 gap-y-5">
            {EventsPanels.map(({ icon, label, type }, index) => (
              <PanelCard
                key={index}
                icon={icon}
                label={label}
                dataType={type}
                onClick={onClick}
              />
            ))}
          </div>
        </div>

        {!allowImport && (
          <div>
            <h3 className="text-ilabel font-ilabel text-text-foreground-secondary mb-2">
              SQL
            </h3>
            <div className="grid grid-cols-2 gap-x-2.5 gap-y-5">
              {SqlPanels.map(({ icon, label, type }, index) => (
                <a
                  key={index}
                  href={generateLinkHref('TABLE', 'SQL', routerQuery)}
                >
                  <PanelCard icon={icon} label={label} dataType={type} />
                </a>
              ))}
            </div>
          </div>
        )}
      </div>
    </SlideOver>
  )
}
