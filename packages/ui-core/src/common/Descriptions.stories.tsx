import '../styles.css'
import type { Story } from '@ladle/react'
import { Descriptions } from './Descriptions'

export const BasicDescriptions: Story = () => {
  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Basic Descriptions</h3>
      <Descriptions
        data={[
          {
            key: 'name',
            label: 'Name',
            value: 'John Doe'
          },
          {
            key: 'age',
            label: 'Age',
            value: '30'
          },
          {
            key: 'email',
            label: 'Email',
            value: 'john.doe@example.com'
          },
          {
            key: 'location',
            label: 'Location',
            value: 'San Francisco, CA'
          }
        ]}
      />
    </div>
  )
}

BasicDescriptions.meta = {
  description: 'Basic description list with label-value pairs'
}

export const NestedObject: Story = () => {
  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Nested Object Values</h3>
      <Descriptions
        data={[
          {
            key: 'user',
            label: 'User',
            value: {
              name: 'Jane Smith',
              email: 'jane@example.com',
              role: 'Admin'
            }
          },
          {
            key: 'company',
            label: 'Company',
            value: 'ACME Corp'
          },
          {
            key: 'address',
            label: 'Address',
            value: {
              street: '123 Main St',
              city: 'New York',
              zip: '10001'
            }
          }
        ]}
      />
    </div>
  )
}

NestedObject.meta = {
  description: 'Descriptions with nested object values'
}

export const EmptyObject: Story = () => {
  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Empty Object</h3>
      <Descriptions
        data={[
          {
            key: 'data',
            label: 'Empty Data',
            value: {}
          },
          {
            key: 'name',
            label: 'Name',
            value: 'Test User'
          }
        ]}
      />
    </div>
  )
}

EmptyObject.meta = {
  description: 'Descriptions showing empty object as { }'
}

export const WithReactElements: Story = () => {
  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">With React Elements</h3>
      <Descriptions
        data={[
          {
            key: 'name',
            label: 'Name',
            value: (
              <span className="font-bold text-blue-600">Alice Johnson</span>
            )
          },
          {
            key: 'status',
            label: 'Status',
            value: (
              <span className="inline-flex items-center rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-800">
                Active
              </span>
            )
          },
          {
            key: 'actions',
            label: 'Actions',
            value: (
              <div className="space-x-2">
                <button className="rounded bg-blue-500 px-2 py-1 text-xs text-white hover:bg-blue-600">
                  Edit
                </button>
                <button className="rounded bg-red-500 px-2 py-1 text-xs text-white hover:bg-red-600">
                  Delete
                </button>
              </div>
            )
          }
        ]}
      />
    </div>
  )
}

WithReactElements.meta = {
  description: 'Descriptions with React elements as values'
}

export const CustomStyling: Story = () => {
  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Custom Styling</h3>
      <Descriptions
        data={[
          {
            key: 'product',
            label: 'Product',
            value: 'Premium Widget'
          },
          {
            key: 'price',
            label: 'Price',
            value: '$99.99'
          },
          {
            key: 'stock',
            label: 'Stock',
            value: '50 units'
          }
        ]}
        labelClassName="text-blue-700 font-bold"
        valueClassName="text-gray-900"
        trClassName="border-b border-gray-200 last:border-0"
        className="rounded border border-gray-300 p-4"
      />
    </div>
  )
}

CustomStyling.meta = {
  description: 'Descriptions with custom className styling'
}

export const WithColon: Story = () => {
  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">With Colon Separator</h3>
      <Descriptions
        data={[
          {
            key: 'server',
            label: 'Server',
            value: 'us-west-1'
          },
          {
            key: 'port',
            label: 'Port',
            value: '8080'
          },
          {
            key: 'protocol',
            label: 'Protocol',
            value: 'HTTPS'
          }
        ]}
        colon={<td className="pr-2 text-gray-400">:</td>}
      />
    </div>
  )
}

WithColon.meta = {
  description: 'Descriptions with colon separator between label and value'
}

export const CustomRenders: Story = () => {
  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Custom Render Functions</h3>
      <Descriptions
        data={[
          {
            key: 'temperature',
            label: 'Temperature',
            value: 25
          },
          {
            key: 'humidity',
            label: 'Humidity',
            value: 65
          },
          {
            key: 'status',
            label: 'Status',
            value: 'online'
          }
        ]}
        renderLabel={(item) => (
          <span className="flex items-center gap-1">
            <span className="h-2 w-2 rounded-full bg-blue-500" />
            {item.label}
          </span>
        )}
        renderValue={(item) => {
          if (item.key === 'temperature') {
            return <span className="text-orange-600">{item.value}°C</span>
          }
          if (item.key === 'humidity') {
            return <span className="text-blue-600">{item.value}%</span>
          }
          return <span className="uppercase text-green-600">{item.value}</span>
        }}
      />
    </div>
  )
}

CustomRenders.meta = {
  description: 'Descriptions with custom render functions for labels and values'
}

export const TransactionData: Story = () => {
  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Complex Transaction Data</h3>
      <Descriptions
        labelClassName="text-gray-600"
        valueClassName="text-gray-800 font-mono text-sm"
        data={[
          {
            key: 'hash',
            label: 'Tx Hash',
            value:
              '0x80b19f0e196a7c05f6b1533fbe13a71746b1795bf7821908b5ce206fded8ad54'
          },
          {
            key: 'from',
            label: 'From',
            value: '0x8434205c1909c8B7ed0D225043bc114d86582ab0'
          },
          {
            key: 'to',
            label: 'To',
            value: '0x81D3877b6b5D8B73C033b54bE0BeC39988E24f26'
          },
          {
            key: 'value',
            label: 'Value',
            value: '0 ETH'
          },
          {
            key: 'gasLimit',
            label: 'Gas Limit',
            value: 636133
          },
          {
            key: 'nonce',
            label: 'Nonce',
            value: 24
          }
        ]}
      />
    </div>
  )
}

TransactionData.meta = {
  description: 'Descriptions showing blockchain transaction data'
}
