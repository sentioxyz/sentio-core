import type { Story } from '@ladle/react'
import { NewSimulation } from './NewSimulation'
import '../styles.css'
import '@sentio/ui-core/dist/style.css'

export const Default: Story = () => {
  return (
    <div className="mx-auto max-w-4xl p-8 text-sm">
      <h2 className="mb-4 text-2xl font-bold">Simulation Form</h2>
      <NewSimulation />
    </div>
  )
}

export const WithDefaultValues: Story = () => {
  return (
    <div className="mx-auto max-w-4xl p-8 text-sm">
      <h2 className="mb-4 text-2xl font-bold">With Default Values</h2>
      <NewSimulation
        defaultValue={{
          from: '0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb',
          blockNumber: 12345678,
          gas: 1000000,
          gasPrice: 100,
          value: 0
        }}
      />
    </div>
  )
}

export const WithCallback: Story = () => {
  return (
    <div className="mx-auto max-w-4xl p-8 text-sm">
      <h2 className="mb-4 text-2xl font-bold">With Callbacks</h2>
      <NewSimulation
        onSuccess={(data) => {
          console.log('Simulation successful:', data)
          alert('Simulation successful!')
        }}
        onChange={(data) => {
          console.log('Form changed:', data)
        }}
      />
    </div>
  )
}

export const WithCustomAPI: Story = () => {
  const mockAPI = async (data: any) => {
    console.log('Mock API called with:', data)
    await new Promise((resolve) => setTimeout(resolve, 1000))
    return {
      simulation: {
        id: 'mock-simulation-id',
        status: 'success'
      }
    }
  }

  return (
    <div className="mx-auto max-w-4xl p-8 text-sm">
      <h2 className="mb-4 text-2xl font-bold">With Custom API</h2>
      <NewSimulation onRequestAPI={mockAPI} />
    </div>
  )
}

export const AllFeatures: Story = () => {
  const mockAPI = async (data: any) => {
    console.log('Mock API called with:', data)
    await new Promise((resolve) => setTimeout(resolve, 1000))
    return {
      simulation: {
        id: 'mock-simulation-id',
        status: 'success'
      }
    }
  }

  return (
    <div className="mx-auto max-w-4xl p-8 text-sm">
      <h2 className="mb-4 text-2xl font-bold">All Features Demo</h2>
      <NewSimulation
        defaultValue={{
          from: '0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb',
          blockNumber: 12345678,
          gas: 1000000,
          gasPrice: 100,
          value: 0,
          header: {
            blockNumber: 12345679,
            blockNumberState: true,
            timestamp: 1640000000,
            timestampState: true
          },
          stateOverride: [
            {
              contract: '0x1234567890123456789012345678901234567890',
              balance: '1000000000000000000',
              storage: [
                {
                  key: '0x0000000000000000000000000000000000000000000000000000000000000000',
                  value:
                    '0x0000000000000000000000000000000000000000000000000000000000000001'
                }
              ]
            }
          ],
          accessList: [
            {
              address: '0x1234567890123456789012345678901234567890',
              storageKeys: [
                '0x0000000000000000000000000000000000000000000000000000000000000000'
              ]
            }
          ]
        }}
        onRequestAPI={mockAPI}
        onSuccess={(data) => {
          console.log('Simulation successful:', data)
          alert('Simulation successful!')
        }}
        onChange={(data) => {
          console.log('Form changed:', data)
        }}
        relatedContracts={[
          {
            address: '0x1234567890123456789012345678901234567890',
            name: 'MyContract'
          }
        ]}
      />
    </div>
  )
}
