import { useFieldArray, useFormContext } from 'react-hook-form'

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
  const { fields, append, remove } = useFieldArray({
    control,
    name: `${name}.storageKeys` as any
  })

  return (
    <div className="dark:bg-sentio-gray-100 rounded border bg-white px-3 py-2">
      <div className="flex items-center gap-1.5">
        <input
          {...register(`${name}.address`)}
          className="border-border-color flex-1 rounded border p-1 font-normal"
          placeholder="Contract Address"
        />
        <button
          type="button"
          className="bg-primary-50 hover:bg-primary-100 rounded px-2 py-1 text-sm"
          onClick={() => append('')}
        >
          +
        </button>
        <button
          type="button"
          className="bg-primary-50 hover:bg-primary-100 rounded px-2 py-1 text-sm"
          onClick={() => onRemove(index)}
        >
          ×
        </button>
      </div>
      <div>
        {fields.map((item, index) => (
          <div
            key={item.id}
            className="group/accesslist flex w-full items-center gap-2 py-1 pl-4"
          >
            <input
              className="border-border-color flex-1 rounded border p-1 font-normal"
              placeholder="Storage key"
              {...register(`${name}.storageKeys.${index}`, {
                required: true
              })}
            />
            <button
              type="button"
              className="bg-primary-50 hover:bg-primary-100 invisible rounded px-2 py-1 text-sm group-hover/accesslist:visible"
              onClick={() => remove(index)}
            >
              ×
            </button>
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
        <button
          type="button"
          className="border-primary hover:bg-primary/10 w-full rounded border px-4 py-2"
          onClick={() => {
            append({
              address: '',
              storageKeys: []
            })
          }}
        >
          + Add address to access list
        </button>
      </div>
    </div>
  )
}
