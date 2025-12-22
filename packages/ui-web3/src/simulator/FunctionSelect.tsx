import { useController, useFormContext, Control } from 'react-hook-form'
import type { AbiFunction } from './types'

interface Props {
  control: Control<any>
  functions?: AbiFunction[]
  className?: string
}

const paramsToDefaultValue = (params: AbiFunction['inputs']) => {
  return params.map((param) => ({
    name: param.name,
    value: ''
  }))
}

export const FunctionSelect = ({ control, functions, className }: Props) => {
  const { field } = useController({
    name: 'function',
    control
  })
  const { setValue } = useFormContext()

  // Separate write and read functions
  const writeFunctions = functions?.filter(
    (f) => !f.stateMutability || !['view', 'pure'].includes(f.stateMutability)
  )
  const readFunctions = functions?.filter(
    (f) => f.stateMutability && ['view', 'pure'].includes(f.stateMutability)
  )

  if (!functions || functions.length === 0) {
    return (
      <div className="cursor-not-allowed rounded-md border border-gray-300 px-4 py-2 text-gray-400">
        Empty functions, please check receiver contract's address or network
      </div>
    )
  }

  return (
    <div className={className}>
      <select
        className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
        value={field.value?.name || ''}
        onChange={(e) => {
          const newFn = functions.find((f) => f.name === e.target.value)
          if (newFn && newFn !== field.value) {
            setValue('functionParams', paramsToDefaultValue(newFn.inputs))
          }
          field.onChange(newFn)
        }}
      >
        <option value="">Select Function</option>
        {writeFunctions && writeFunctions.length > 0 && (
          <optgroup label="Write">
            {writeFunctions.map((fn) => (
              <option key={fn.name} value={fn.name}>
                {fn.name}
              </option>
            ))}
          </optgroup>
        )}
        {readFunctions && readFunctions.length > 0 && (
          <optgroup label="Read">
            {readFunctions.map((fn) => (
              <option key={fn.name} value={fn.name}>
                {fn.name}
              </option>
            ))}
          </optgroup>
        )}
      </select>
      {field.value && (
        <div className="mt-2 rounded-md bg-gray-50 p-3 text-xs">
          <div className="font-semibold text-gray-900">{field.value.name}</div>
          {field.value.stateMutability && (
            <div className="mt-1 text-gray-600">
              State Mutability:{' '}
              <span className="font-medium">{field.value.stateMutability}</span>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
