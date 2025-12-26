import { Suspense, useEffect } from 'react'
import { useAtomValue } from 'jotai'
import { Input } from '@sentio/ui-core'
import { useFormContext } from 'react-hook-form'
import { blockSummary, simulationFormState } from './atoms'

const emptyFn: any = () => {}

const _Input = ({
  name,
  latestIndex
}: {
  name: string
  latestIndex?: number
}) => {
  const {
    register,
    setValue,
    formState: { errors }
  } = useFormContext()
  const atomFormState = useAtomValue(simulationFormState)

  useEffect(() => {
    if (atomFormState.usePendingBlock && latestIndex) {
      setValue(name, latestIndex)
    }
  }, [atomFormState.usePendingBlock, latestIndex, name, setValue])

  return (
    <div className="space-y-2">
      <div className="text-ilabel text-text-foreground font-medium">
        Position in Block (transaction index)
      </div>
      <div>
        {atomFormState.usePendingBlock ? (
          <Input
            key="b1"
            name=""
            value="/"
            onChange={emptyFn}
            onBlur={emptyFn}
            disabled
          />
        ) : (
          <Input
            error={errors[name]}
            {...register(name, {
              required: true,
              valueAsNumber: true,
              min: {
                value: 0,
                message: 'Cannot be less than 0.'
              },
              max: {
                value: latestIndex || Number.MAX_VALUE,
                message:
                  'Should less or equal than the latest position in block'
              }
            })}
            className="border-border-color w-full rounded-md border p-2 font-normal"
          />
        )}
      </div>
      {latestIndex ? (
        <div className="text-gray text-xs">Latest Postion: {latestIndex}</div>
      ) : null}
    </div>
  )
}

const _TxnNumberInput = ({ name }: { name: string }) => {
  const bsummary = useAtomValue(blockSummary)
  return <_Input latestIndex={bsummary.transactionCount} name={name} />
}

export const TxnNumberInput = ({ name = 'txIndex' }: { name?: string }) => {
  return (
    <Suspense fallback={<_Input latestIndex={0} name={name} />}>
      <_TxnNumberInput name={name} />
    </Suspense>
  )
}
