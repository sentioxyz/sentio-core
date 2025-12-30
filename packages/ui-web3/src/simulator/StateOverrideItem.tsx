import { useState, useCallback } from 'react'
import {
  useFormContext,
  useFieldArray,
  useWatch,
  useController
} from 'react-hook-form'
import {
  ChevronDownIcon,
  ChevronRightIcon,
  PlusIcon,
  QuestionMarkCircleIcon,
  TrashIcon
} from '@heroicons/react/24/outline'
import { CheckIcon, XMarkIcon } from '@heroicons/react/20/solid'
import { Disclosure } from '@headlessui/react'
import { PopoverTooltip, Button as NewButton } from '@sentio/ui-core'
import { AddressAvatar } from './ContractName'
import { ContractInput } from './ContractInput'
import { ContractAddress } from '../transaction/ContractComponents'

interface Props {
  name: string
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

const StorageInput = ({
  onSave,
  onCancel
}: {
  onSave: (payload: StorageVariable) => void
  onCancel: () => void
}) => {
  const [key, setKey] = useState('')
  const [value, setValue] = useState('')
  const isValid = key && value

  const onClose = useCallback(() => {
    onCancel()
    setKey('')
    setValue('')
  }, [onCancel])

  const onConfirm = () => {
    if (key && value) {
      onSave({
        key,
        value
      })
      onClose()
    }
  }

  return (
    <form
      onSubmit={(e) => {
        e.stopPropagation()
        e.preventDefault()
        onConfirm()
      }}
    >
      <div className="flex items-center justify-between gap-2">
        <input
          className="border-border-color w-[145px] rounded border px-2 py-[5px] font-normal"
          value={key}
          onChange={(e) => {
            setKey(e.target.value)
          }}
          placeholder="Key"
        />
        <input
          className="border-border-color flex-1 rounded border px-2 py-[5px] font-normal"
          value={value}
          onChange={(e) => {
            setValue(e.target.value)
          }}
          placeholder="Value"
        />
        <NewButton
          size="md"
          role="custom"
          className="cursor-pointer bg-cyan-100 hover:bg-cyan-200"
          disabled={!isValid}
          icon={<CheckIcon className="text-cyan-600" />}
          onClick={(evt) => {
            evt.stopPropagation()
            onConfirm()
          }}
        />
        <NewButton
          size="md"
          icon={<XMarkIcon />}
          onClick={() => {
            onClose()
          }}
          role="text"
        />
      </div>
    </form>
  )
}

const StorageVariableItem = ({
  data,
  onRemove
}: {
  data: StorageVariable
  onRemove: () => void
}) => {
  return (
    <div className="flex items-center gap-2">
      <div className="w-[145px] rounded border px-2 py-1">
        <input
          disabled
          value={data.key}
          className="dark:!bg-sentio-gray-100 w-full border-0 !bg-white"
        />
      </div>
      <div className="flex-1 rounded border px-2 py-1">
        <input
          disabled
          value={data.value}
          className="dark:!bg-sentio-gray-100 w-full border-0 !bg-white"
        />
      </div>
      <div className="flex-0">
        <NewButton
          icon={<TrashIcon />}
          size="md"
          onClick={onRemove}
          role="text"
        />
      </div>
    </div>
  )
}

export const StateOverrideItem = ({
  name,
  onRemove,
  index,
  relatedContracts
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
      shouldUnregister: true,
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
    remove
  } = useFieldArray({
    control,
    name: `${name}.storage`
  })
  const [showAdding, setShowAdding] = useState(false)
  const useCustomBalance = useWatch({
    control,
    name: `${name}.customBalance`
  })

  return (
    <div className="flex w-full gap-2">
      <Disclosure defaultOpen>
        <div className="dark:bg-sentio-gray-100 border-border-color relative flex-1 space-y-4 rounded border bg-white px-3 py-2.5">
          <Disclosure.Button
            as="div"
            className="flex w-full items-center gap-2"
          >
            {({ open }) => (
              <>
                <AddressAvatar name={contractAddress || '...'} />
                <div className="flex-1">
                  <ContractAddress address={contractAddress} />
                </div>
                {open ? (
                  <ChevronDownIcon className="text-gray h-3.5 w-3.5" />
                ) : (
                  <ChevronRightIcon className="text-gray h-3.5 w-3.5" />
                )}
              </>
            )}
          </Disclosure.Button>
          <Disclosure.Panel className="space-y-4">
            <div className="space-y-2">
              <div className="text-ilabel font-medium">Address</div>
              <div>
                <ContractInput
                  name={`${name}.contract`}
                  placeholder="Please provide the contract address for overriding"
                  relatedContracts={relatedContracts}
                />
              </div>
            </div>
            <div className="space-y-2">
              <div className="flex w-full justify-between">
                <div className="text-ilabel font-medium">Balance</div>
                <div
                  className="text-primary active:text-primary-700 cursor-pointer"
                  onClick={() => {
                    setValue(`${name}.customBalance`, !useCustomBalance)
                  }}
                >
                  {useCustomBalance ? 'Use Default' : 'Use Custom'}
                </div>
              </div>
              <div className="relative">
                <input
                  {...register(`${name}.balance`, {
                    disabled: !useCustomBalance
                  })}
                  placeholder={useCustomBalance ? 'input balance' : '/'}
                  className="border-border-color w-full rounded border p-2 pr-14 font-normal"
                />
                {useCustomBalance ? (
                  <span className="text-gray absolute right-4 top-2.5 font-medium">
                    wei
                  </span>
                ) : null}
              </div>
            </div>
            <div className="space-y-2">
              <div className="text-ilabel font-medium">Storage Variables</div>
              <div className="text-gray flex gap-1 text-xs">
                Storage Variable keys and values can be added as encoded or
                decoded.
                <PopoverTooltip
                  icon={
                    <QuestionMarkCircleIcon className="h-3.5 w-3.5 cursor-pointer" />
                  }
                  text={
                    <span className="text-gray text-xs">
                      Storage keys must be simple types (e.g. int, unit, bool,
                      address, bytes). Tuple fields can be accessed using dot
                      (e.g. tuple.name).
                    </span>
                  }
                />
              </div>
              <div>
                <div className="pb-2">
                  {storageVariables.map((item, index) => {
                    return (
                      <StorageVariableItem
                        key={item.id}
                        data={item as any}
                        onRemove={() => {
                          remove(index)
                        }}
                      />
                    )
                  })}
                </div>
                {showAdding ? (
                  <StorageInput
                    onSave={(payload) => {
                      append(payload)
                    }}
                    onCancel={() => setShowAdding(false)}
                  />
                ) : (
                  <NewButton
                    size="md"
                    icon={<PlusIcon />}
                    onClick={() => setShowAdding(true)}
                  >
                    Add More
                  </NewButton>
                )}
              </div>
            </div>
          </Disclosure.Panel>
        </div>
      </Disclosure>
      <div className="shrink-0 pt-3">
        <TrashIcon
          className="hover:text-red h-[18px] w-[18px] cursor-pointer"
          onClick={() => {
            onRemove(index)
          }}
        />
      </div>
    </div>
  )
}
