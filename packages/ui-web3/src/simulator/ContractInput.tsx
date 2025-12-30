import { useState } from 'react'
import { Menu } from '@headlessui/react'
import { useFormContext, useFormState, useWatch } from 'react-hook-form'
import { Input } from '@sentio/ui-core'
import get from 'lodash/get'
import classNames from 'classnames'

const contractAddressRegex = /^0x[a-fA-F0-9]{40}$/

interface Props {
  name: string
  placeholder?: string
  relatedContracts?: {
    address: string
    name: string
  }[]
}

export const ContractInput = ({
  name,
  placeholder,
  relatedContracts
}: Props) => {
  const { register, setValue, control } = useFormContext()
  const { errors } = useFormState({
    control
  })
  const [openMenu, setOpenMenu] = useState(false)
  const receiverAddress = useWatch({
    control: control,
    name: 'contract.address'
  })

  return (
    <Menu as="div" className="group relative">
      <Input
        error={get(errors, name) as any}
        autoComplete="off"
        placeholder={placeholder}
        className="text-icontent border-border-color w-full rounded-md border p-2 font-normal"
        {...register(name, {
          required: true,
          minLength: {
            value: 42,
            message: 'Contract Address should contain 42 characters.'
          },
          maxLength: {
            value: 42,
            message:
              'Contract Address should not contain more than 42 characters.'
          },
          pattern: {
            value: contractAddressRegex,
            message: 'Contract address is not valid, please check again.'
          }
        })}
        onFocus={() => {
          setOpenMenu(true)
        }}
      />
      {openMenu && (
        <Menu.Items
          className="dark:bg-sentio-gray-100 absolute z-[1] hidden max-h-40 w-full translate-y-1 overflow-auto rounded-md border bg-white font-normal shadow group-focus-within:block"
          static
        >
          <div className="text-gray px-2 py-2 text-xs">
            Alternatively, select from the related addresses:
          </div>
          {receiverAddress && (
            <Menu.Item>
              {({ active }) => (
                <div
                  onClick={() => {
                    setValue(name, receiverAddress, { shouldValidate: true })
                    setOpenMenu(false)
                  }}
                  className={classNames(
                    'flex w-full items-center justify-between px-2 py-1.5',
                    active ? 'bg-gray-100' : ''
                  )}
                >
                  <span className="font-mono text-xs">{receiverAddress}</span>
                  <span className="bg-primary-800/80 rounded-md px-2 py-0.5 text-white">
                    Receiver
                  </span>
                </div>
              )}
            </Menu.Item>
          )}
          {relatedContracts?.map((item) => {
            if (item.address === receiverAddress) {
              return null
            }
            return (
              <Menu.Item key={item.address}>
                {({ active }) => (
                  <div
                    onClick={() => {
                      setValue(name, item.address, { shouldValidate: true })
                      setOpenMenu(false)
                    }}
                    className={classNames(
                      'flex w-full items-center justify-between px-2 py-1.5',
                      active ? 'bg-gray-100' : ''
                    )}
                  >
                    <span className="font-mono text-xs">{item.address}</span>
                    <span className="text-primary-800/80">{item.name}</span>
                  </div>
                )}
              </Menu.Item>
            )
          })}
        </Menu.Items>
      )}
    </Menu>
  )
}
