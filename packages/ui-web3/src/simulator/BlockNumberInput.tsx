import { Suspense, useEffect } from 'react'
import { useFormContext } from 'react-hook-form'
import { Input } from '@sentio/ui-core'
import { useSimulatorContext } from './SimulatorContext'

const emptyFn: any = () => {}

const parseHex = (hex?: string) => {
  if (!hex) return 0
  return parseInt(hex, 16)
}

const _Input = ({ latestIndex }: { latestIndex?: number }) => {
  const {
    register,
    formState: { errors },
    watch,
    setValue
  } = useFormContext()
  const { simulationFormState: atomFormState, setBlockNumber } =
    useSimulatorContext()
  const bIndex = watch('blockNumber')

  useEffect(() => {
    if (bIndex) {
      setBlockNumber(parseInt(bIndex))
    }
  }, [bIndex, setBlockNumber])

  useEffect(() => {
    if (atomFormState.usePendingBlock && latestIndex) {
      setValue('blockNumber', latestIndex)
      setBlockNumber(latestIndex)
    }
  }, [atomFormState.usePendingBlock, latestIndex, setValue, setBlockNumber])

  return (
    <div className="space-y-2">
      <div className="text-ilabel text-text-foreground font-medium">
        Block Number
      </div>
      <div>
        {atomFormState.usePendingBlock ? (
          <Input
            key="b1"
            name=""
            value="/"
            onChange={emptyFn}
            onBlur={emptyFn}
            disabled={true}
          />
        ) : (
          <Input
            error={errors.blockNumber as any}
            {...register('blockNumber', {
              required: true,
              min: {
                value: 1,
                message: 'Block number canot be less than 1.'
              },
              max: {
                value: latestIndex || Number.MAX_VALUE,
                message:
                  'Block number should less or equal than the latest block number'
              }
            })}
          />
        )}
      </div>
      {latestIndex ? (
        <div className="text-gray text-xs">Latest Block: {latestIndex}</div>
      ) : null}
    </div>
  )
}

const _BlockNumberInput = () => {
  const { latestBlockNumber } = useSimulatorContext()
  const { blockNumber } = latestBlockNumber
  const latestIndex = Number(parseHex(blockNumber || '')) || 0
  return <_Input latestIndex={latestIndex} />
}

export const BlockNumberInput = () => {
  return (
    <Suspense fallback={<_Input latestIndex={0} />}>
      <_BlockNumberInput />
    </Suspense>
  )
}
