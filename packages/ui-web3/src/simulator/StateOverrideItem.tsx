import { useState, useCallback, useEffect, useMemo } from 'react'
import {
  useFormContext,
  useFieldArray,
  useWatch,
  useController
} from 'react-hook-form'

interface Contract {
  address?: string
  name?: string
  chainId?: string
}

interface Props {
  name: string
  contracts: Contract[]
  index: number
  onRemove: (index?: number) => void
  relatedContracts?: {
    address: string
    name: string
  }[]
}

type StorageVariable = {
  key: string
  value: string
}

const contractAddressRegex = /^0x[a-fA-F0-9]{40}$/

export const StateOverrideItem = ({
  name,
  contracts,
  onRemove,
  index
}: Props) => {
  const { register, control, setValue } = useFormContext()
  const {
    field: { value: contractAddress },
    fieldState: { error }
  } = useController({
    name: `${name}.contract`,
    control,
    rules: {
      required: true,
      minLength: {
        value: 42,
        message: 'Contract Address should contain 42 characters.'
      },
      maxLength: {
        value: 42,
        message: 'Contract Address should not contain more than 42 characters.'
      },
      pattern: {
        value: contractAddressRegex,
        message: 'Contract address is not valid, please check again.'
      }
    }
  })

  const {
    fields: storageVariables,
    append,
    remove: removeStorage
  } = useFieldArray({
    control,
    name: `${name}.storage`
  })

  return (
    <div className="border-border-color relative space-y-4 rounded border bg-white px-3 py-2.5">
      <div className="space-y-2">
        <label className="text-sm font-medium">Contract Address</label>
        <input
          {...register(`${name}.contract`)}
          className="border-border-color w-full rounded border p-2 font-mono"
          placeholder="0x..."
        />
        {error && <span className="text-xs text-red-600">{error.message}</span>}
      </div>

      <div className="space-y-2">
        <label className="text-sm font-medium">Balance Override</label>
        <input
          {...register(`${name}.balance`)}
          className="border-border-color w-full rounded border p-2"
          placeholder="Balance in Wei"
        />
      </div>

      <div className="space-y-2">
        <label className="text-sm font-medium">Storage Overrides</label>
        {storageVariables.map((item, idx) => (
          <div key={item.id} className="flex gap-2">
            <input
              {...register(`${name}.storage.${idx}.key`)}
              className="border-border-color w-1/3 rounded border p-2 font-mono"
              placeholder="Storage Key"
            />
            <input
              {...register(`${name}.storage.${idx}.value`)}
              className="border-border-color flex-1 rounded border p-2 font-mono"
              placeholder="Storage Value"
            />
            <button
              type="button"
              onClick={() => removeStorage(idx)}
              className="px-2 text-red-600 hover:text-red-800"
            >
              ×
            </button>
          </div>
        ))}
        <button
          type="button"
          onClick={() => append({ key: '', value: '' })}
          className="text-primary-600 hover:text-primary-800 text-sm"
        >
          + Add Storage Override
        </button>
      </div>

      <button
        type="button"
        onClick={() => onRemove(index)}
        className="absolute -right-2 -top-2 rounded-full border bg-gray-100 px-2 py-1 hover:bg-red-100"
      >
        ×
      </button>
    </div>
  )
}
