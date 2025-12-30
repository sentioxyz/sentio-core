import { useFieldArray, Control } from 'react-hook-form'
import { SimulationFormType } from './types'
import { StateOverrideItem } from './StateOverrideItem'
import { Button } from '@sentio/ui-core'
import { PlusIcon } from '@heroicons/react/24/outline'

interface Contract {
  address?: string
  name?: string
  chainId?: string
}

interface Props {
  control: Control<SimulationFormType>
  relatedContracts?: {
    address: string
    name: string
  }[]
}

export const StateOverride = ({ control, relatedContracts }: Props) => {
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
            onRemove={remove}
            index={index}
            name={name}
            relatedContracts={relatedContracts}
          />
        )
      })}
      <div>
        <Button
          className="!border-primary hover:bg-primary/10 !border"
          role="link"
          size="md"
          onClick={() => {
            append({
              contract: '',
              balance: '',
              storage: []
            })
          }}
          icon={<PlusIcon />}
        >
          Add State Override
        </Button>
      </div>
    </div>
  )
}
