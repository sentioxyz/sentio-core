import { useFieldArray, useWatch, useFormContext } from 'react-hook-form'
import { Button as NewButton } from '@sentio/ui-core'
import { AddressAvatar } from './ContractName'
import {
  CircleStackIcon,
  PlusIcon,
  XMarkIcon
} from '@heroicons/react/24/outline'

const AccessListItem = ({
  name,
  onRemove,
  index
}: {
  name: `accessList.${number}`
  onRemove: (index?: number) => void
  index: number
}) => {
  const { register, control } = useFormContext()
  const address = useWatch({
    control,
    name: `${name}.address` as any
  })
  const { fields, append, remove } = useFieldArray({
    control,
    name: `${name}.storageKeys` as any
  })
  return (
    <div className="dark:bg-sentio-gray-100 rounded border bg-white px-3 py-2">
      <div className="flex items-center gap-1.5">
        <AddressAvatar name={address} />
        <input
          {...register(`${name}.address`)}
          className="border-border-color flex-1 rounded border p-1 font-normal"
          placeholder="Contract Address"
        />
        <section className="inline-block space-x-2">
          <NewButton
            icon={<CircleStackIcon className="text-primary" />}
            size="sm"
            role="custom"
            className="bg-primary-50 hover:bg-primary-100/80 active:bg-primary-100"
            onClick={() => {
              append('')
            }}
          ></NewButton>
          <NewButton
            icon={<XMarkIcon className="text-primary" />}
            size="sm"
            role="custom"
            className="bg-primary-50 hover:bg-primary-100/80 active:bg-primary-100"
            onClick={() => {
              onRemove(index)
            }}
          ></NewButton>
        </section>
      </div>
      <div>
        {fields.map((item, index) => (
          <div
            key={item.id}
            className="group/accesslist flex w-full items-center gap-2 py-1 pl-4"
          >
            <CircleStackIcon className="text-gray h-4 w-4" />
            <input
              className="border-border-color flex-1 rounded border p-1 font-normal"
              placeholder="Storage key"
              {...register(`${name}.storageKeys.${index}`, {
                required: true
              })}
            />
            <NewButton
              size="sm"
              role="custom"
              className="bg-primary-50 hover:bg-primary-100/80 active:bg-primary-100 invisible group-hover/accesslist:visible"
              icon={<XMarkIcon className="text-primary" />}
              onClick={() => {
                remove(index)
              }}
            />
          </div>
        ))}
      </div>
    </div>
  )
}

export const OptionalAccessList = () => {
  const { control } = useFormContext()
  const { fields, append, remove } = useFieldArray({
    control,
    name: 'accessList'
  })

  return (
    <div className="space-y-3">
      <div className="space-y-2">
        {fields.map((item, index) => (
          <AccessListItem
            key={item.id}
            name={`accessList.${index}`}
            onRemove={remove}
            index={index}
          />
        ))}
      </div>
      <div>
        <NewButton
          className="!border-primary hover:bg-primary/10 !border"
          size="md"
          onClick={() => {
            append({
              address: '',
              storageKeys: []
            })
          }}
          role="link"
          icon={<PlusIcon />}
        >
          Add address to access list
        </NewButton>
      </div>
    </div>
  )
}
