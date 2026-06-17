import type { Story } from '@ladle/react'
import { useState } from 'react'
import { FunctionInput } from './FunctionInput'
import type { QueryLike } from '../types/metrics'

export const Basic: Story = () => {
  const [value, setValue] = useState<QueryLike>({
    query: 'erc20.transfer',
    functions: []
  })
  return (
    <div className="w-[36rem] p-8">
      <div className="flex flex-wrap items-center gap-y-2">
        <FunctionInput value={value} onChange={setValue} />
      </div>
      <pre className="text-text-foreground-secondary mt-4 text-xs">
        {JSON.stringify(value.functions ?? [], null, 2)}
      </pre>
    </div>
  )
}
Basic.meta = {
  description: 'Chained query functions — add via f(x), edit args inline'
}
