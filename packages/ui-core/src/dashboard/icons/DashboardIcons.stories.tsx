import '../../styles.css'
import {
  LinesIcon,
  AreasIcon,
  BarsIcon,
  GaugeIcon,
  PieIcon,
  TableIcon,
  NoteIcon,
  QueryValueIcon,
  EventLogsIcon,
  SQlIcon,
  RetentionIcon,
  GroupsIcon,
  ImportIcon,
  ScatterIcon,
  ShellIcon
} from './index'

// Ladle renders exported React components from *.stories.* files.
const icons: Array<[string, React.ReactNode]> = [
  ['LinesIcon', <LinesIcon />],
  ['AreasIcon', <AreasIcon />],
  ['BarsIcon', <BarsIcon />],
  ['GaugeIcon', <GaugeIcon />],
  ['PieIcon', <PieIcon />],
  ['TableIcon', <TableIcon />],
  ['NoteIcon', <NoteIcon />],
  ['QueryValueIcon', <QueryValueIcon />],
  ['EventLogsIcon', <EventLogsIcon />],
  ['SQlIcon', <SQlIcon />],
  ['RetentionIcon', <RetentionIcon />],
  ['GroupsIcon', <GroupsIcon />],
  ['ScatterIcon', <ScatterIcon />],
  ['ImportIcon', <ImportIcon className="h-14 w-14" />],
  ['ShellIcon', <ShellIcon className="h-14 w-14" />]
]

export const AllIcons = () => (
  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 24 }}>
    {icons.map(([name, node]) => (
      <div key={name} className="flex w-24 flex-col items-center">
        {node}
        <div className="text-icontent mt-1 text-center">{name}</div>
      </div>
    ))}
  </div>
)
