import type { Story } from '@ladle/react'
import { useState } from 'react'
import { ArgumentInput } from './ArgumentInput'
import { ArgumentType } from './functions'
import type { ArgumentLike } from '../types/metrics'

export const Duration: Story = () => {
  const [value, setValue] = useState<ArgumentLike>({
    durationValue: { value: 1, unit: 'm' }
  })
  return (
    <div className="w-72 p-8">
      <ArgumentInput
        argument={{ name: 'interval', type: ArgumentType.Duration }}
        value={value}
        onChange={setValue}
      />
      <p className="text-text-foreground-secondary mt-4 text-sm">
        {value.durationValue?.value} {value.durationValue?.unit}
      </p>
    </div>
  )
}
Duration.meta = { description: 'Duration argument → DurationInput' }

export const IntegerArg: Story = () => {
  const [value, setValue] = useState<ArgumentLike>({ intValue: 5 })
  return (
    <div className="w-72 p-8">
      <ArgumentInput
        argument={{ name: 'k', type: ArgumentType.Integer }}
        value={value}
        onChange={setValue}
        className="border-main w-24 rounded-md border"
      />
      <p className="text-text-foreground-secondary mt-4 text-sm">
        k = {value.intValue}
      </p>
    </div>
  )
}
IntegerArg.meta = { description: 'Integer argument → number input' }
