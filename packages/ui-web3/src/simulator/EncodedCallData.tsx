import { Suspense } from 'react'
import { Control, useController } from 'react-hook-form'
import { ClipLoader } from 'react-spinners'
import { DebounceInput } from 'react-debounce-input'

interface Props {
  control: Control<any>
  className?: string
  type?: 'encoded' | 'decoded'
}

const EncodedParams = ({ control }: { control: Props['control'] }) => {
  const { field } = useController({
    name: 'input',
    control,
    defaultValue: ''
  })
  return (
    <DebounceInput
      rows={5}
      value={field.value}
      onChange={(e) => field.onChange(e.target.value)}
      element="textarea"
      className="text-icontent border-border-color w-full rounded-md font-mono font-normal placeholder:font-sans"
      debounceTimeout={300}
      placeholder="Input decoded call data here"
    />
  )
}

export const EncodedCallData = ({ control, className }: Props) => {
  const cn = `mt-4 space-y-4 text-xs font-medium ${className || ''}`

  return (
    <div className={cn}>
      <Suspense
        fallback={
          <div className="flex h-40 w-full items-center justify-center gap-2">
            <ClipLoader
              loading
              color="#3B82F6"
              size={24}
              cssOverride={{
                borderWidth: 3
              }}
            />
          </div>
        }
      >
        <EncodedParams control={control} />
      </Suspense>
    </div>
  )
}
