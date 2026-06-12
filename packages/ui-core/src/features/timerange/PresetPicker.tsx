import { now, ago, DateTimeValue, previous } from '../../utils/time'

const presets = {
  // 'Past 5 minutes': [ago(5, 'minutes'), now],
  'Past 15 minutes': [ago(15, 'minutes'), now],
  'Past hour': [ago(1, 'hours'), now],
  // 'Past 4 hours': [ago(4, 'hours'), now],
  'Past 12 hours': [ago(12, 'hours'), now],
  'Past 1 day': [ago(1, 'days'), now],
  'Past 7 days': [ago(7, 'days'), now],
  'Past 1 month': [ago(1, 'months'), now],
  'Past 3 month': [ago(3, 'months'), now],
  'Past 6 months': [ago(6, 'months'), now],
  'Past year': [ago(1, 'years'), now],
  'This Week': [previous(0, 'weeks'), previous(0, 'weeks')],
  'This Month': [previous(0, 'months'), previous(0, 'months')],
  // 'This Year': [previous(0, 'years'), previous(0, 'years')],
  'Previous Week': [previous(1, 'weeks'), previous(1, 'weeks')],
  'Previous Month': [previous(1, 'months'), previous(1, 'months')]
  // 'Previous Year': [previous(1, 'years'), previous(1, 'years')]
}

interface Props {
  onSelect: (start: DateTimeValue, end: DateTimeValue) => void
}

export function PresetPicker({ onSelect }: Props) {
  return (
    <div
      aria-labelledby="quick-select-presets"
      className="w-37 absolute bottom-0 left-0 top-0 overflow-y-auto border-r p-4"
    >
      <div className="flex flex-col gap-1.5">
        {Object.entries(presets).map(([label, [from, to]]) => (
          <button
            key={label}
            onClick={() => onSelect(from, to)}
            className="hover:border-primary-600 hover:bg-primary-600 text-text-foreground text-icontent h-fit w-full whitespace-nowrap rounded-full border px-3 py-1 hover:text-white sm:w-fit sm:text-xs"
          >
            {label}
          </button>
        ))}
      </div>
    </div>
  )
}
