import { useController, useFormContext, Control } from 'react-hook-form'
import type { AbiFunction, FunctionType } from './types'
import { Select } from '@sentio/ui-core'
import { FunctionOption } from './FunctionOption'
import { useSimulatorContext } from './SimulatorContext'
import { useEffect, useState } from 'react'
import isEqual from 'lodash/isEqual'

const SelectComponent = Select<FunctionType>

interface Props {
  control: Control<any>
}

const paramsToDefaultValue = (params: FunctionType['inputs']) => {
  const res: {
    name: string
    value: string
  }[] = []
  for (const param of params) {
    res.push({
      name: param.name,
      value: ''
    })
  }
  return res
}

const groupedOrder = [
  {
    key: 'Writable',
    label: 'Write'
  },
  {
    key: 'Readonly',
    label: 'Read'
  }
]

export const FunctionSelect = ({ control }: Props) => {
  const { field } = useController({
    name: 'function',
    control
  })
  const { setValue } = useFormContext()
  const { contractFunctions } = useSimulatorContext()
  const { wfunctions, rfunctions } = contractFunctions || {}
  console.log('FunctionSelect functions', contractFunctions, field.value)
  const [options, setOptions] = useState<
    {
      label: string
      value: any
      group: string
    }[]
  >([])

  useEffect(() => {
    if (!wfunctions && !rfunctions) {
      return
    }
    setOptions((pre) => {
      const current = [
        ...(wfunctions || []).map((item) => ({
          label: item.name,
          value: item,
          group: 'Writable'
        })),
        ...(rfunctions || []).map((item) => ({
          label: item.name,
          value: item,
          group: 'Readonly'
        }))
      ]
      if (isEqual(pre, current)) {
        return pre
      }
      return current
    })
  }, [wfunctions, rfunctions])

  if (!wfunctions && !rfunctions) {
    return (
      <div className="text-icontent cursor-not-allowed rounded-md border border-gray-300 px-4 py-2 text-gray-400">
        Empty functions, please check receiver contract's address or network
      </div>
    )
  }

  return (
    <SelectComponent
      placeholder="Select Function"
      size="md"
      options={options}
      groupedOptions={true}
      unmountOptions={false}
      groupedOrder={groupedOrder}
      value={field.value as any}
      onChange={(newFn) => {
        if (newFn !== field.value) {
          // reset function params
          setValue('functionParams', paramsToDefaultValue(newFn.inputs))
        }
        field.onChange(newFn)
      }}
      renderOption={(option, state) => {
        return <FunctionOption data={option.value} {...state} />
      }}
      noOptionsMessage={
        <div>
          <div className="text-primary-800 text-sm font-medium">
            No Functions
          </div>
          <div className="text-sm text-gray-500">
            This contract does not have any function
          </div>
        </div>
      }
    />
  )
}
