import { useFieldArray, Control } from 'react-hook-form'
import { SimulationFormType } from './types'
import { StateOverrideItem } from './StateOverrideItem'

interface Contract {
  address?: string
  name?: string
  chainId?: string
}

interface Props {
  control: Control<SimulationFormType>
  contracts?: Contract[]
  relatedContracts?: {
    address: string
    name: string
  }[]
}

export const StateOverride = ({
  control,
  contracts,
  relatedContracts
}: Props) => {
  const { fields, append, remove } = useFieldArray({
    name: 'stateOverride',
    control
  })

  return (
    <div className="mt-2 space-y-3">
      <div className="text-xs text-gray-800">
        Add contracts and set state overrides.
      </div>
      {fields.map((item, index) => {
        const name = `stateOverride.${index}`
        return (
          <StateOverrideItem
            key={item.id}
            contracts={contracts || []}
            onRemove={remove}
            index={index}
            name={name}
            relatedContracts={relatedContracts}
          />
        )
      })}
      <div>
        <button
          type="button"
          className="border-primary hover:bg-primary/10 w-full rounded border px-4 py-2"
          onClick={() => {
            append({
              contract: '',
              balance: '',
              storage: []
            })
          }}
        >
          + Add State Override
        </button>
      </div>
    </div>
  )
}
