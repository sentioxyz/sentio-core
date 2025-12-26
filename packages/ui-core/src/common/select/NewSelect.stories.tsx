import type { Story } from '@ladle/react'
import { Select } from './Select'
import { cx } from 'class-variance-authority'
import { ReactNode, useState } from 'react'
import '../../styles.css'

export const Primary: Story = () => {
  const [value, setValue] = useState('hex')

  return (
    <div className="w-48">
      <Select
        options={[
          { label: 'Hex', value: 'hex' },
          { label: 'Number', value: 'number' }
        ]}
        value={value}
        onChange={setValue}
      />
    </div>
  )
}

export const CustomizeLabel: Story = () => {
  const [value, setValue] = useState('hex')

  return (
    <div className="w-48">
      <Select
        options={[
          {
            label: ({ active }) => (
              <div className="flex w-full items-center justify-between px-2">
                <span
                  className={cx(
                    'text-primary-800 rounded-full px-2 py-1 text-sm',
                    active ? 'dark:bg-sentio-gray-100 bg-white' : 'bg-gray-100'
                  )}
                >
                  Hex
                </span>
                {active ? (
                  <span className="text-white">Hovered!</span>
                ) : (
                  <span
                    className={cx(
                      'text-xs',
                      active ? 'text-white' : 'text-gray-600'
                    )}
                  >
                    raw format
                  </span>
                )}
              </div>
            ),
            value: 'hex'
          },
          {
            label: 'Number',
            value: 'number'
          }
        ]}
        value={value}
        onChange={setValue}
      />
    </div>
  )
}

export const CustomizeRender: Story = () => {
  const [value, setValue] = useState('hex')

  return (
    <div className="w-48">
      <Select
        options={[
          { label: 'Hex', value: 'hex' },
          { label: 'Number', value: 'number' }
        ]}
        value={value}
        onChange={setValue}
        renderOption={(option, state) => (
          <div className="flex w-full items-center justify-between px-2">
            <span
              className={cx(
                'text-primary-800 rounded-full px-2 py-1 text-sm',
                state.active
                  ? 'dark:bg-sentio-gray-100 bg-white'
                  : 'bg-gray-100'
              )}
            >
              {option.label as ReactNode}
            </span>
            <span
              className={cx(
                'text-xs',
                state.active ? 'text-white' : 'text-gray-600'
              )}
            >
              raw format
            </span>
          </div>
        )}
      />
    </div>
  )
}
