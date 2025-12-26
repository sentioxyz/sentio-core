import InputLabel from '@mui/material/InputLabel'
import MenuItem from '@mui/material/MenuItem'
import FormControl from '@mui/material/FormControl'
import Select, { SelectChangeEvent } from '@mui/material/Select'
import dayjs from 'dayjs'
import localizedFormat from 'dayjs/plugin/localizedFormat'
import relativeTime from 'dayjs/plugin/relativeTime'

dayjs.extend(localizedFormat)
dayjs.extend(relativeTime)

interface Props<T> {
  simulates: T &
    {
      id: string
      name: string
      createdAt: string
    }[]
  value: string
  onChange: (v: string) => void
}

export const SimulateHistory = ({ simulates, value, onChange }: Props<any>) => {
  return (
    <FormControl sx={{ m: 1, minWidth: 160 }} size="small">
      <InputLabel id="simulate-history-label">History</InputLabel>
      <Select
        labelId="simulate-history-label"
        id="simulate-history-select"
        value={value}
        label="History"
        onChange={(e: SelectChangeEvent) => onChange(e.target.value)}
      >
        {simulates.map((item) => (
          <MenuItem
            value={item.id}
            style={{ fontSize: 12 }}
            className="flex w-full justify-between gap-8"
            title={`${item.name} (${dayjs(item.createdAt).format('LTS')})`}
          >
            <span>{item.name}</span>
            <span>{dayjs(item.createdAt).fromNow()}</span>
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  )
}
