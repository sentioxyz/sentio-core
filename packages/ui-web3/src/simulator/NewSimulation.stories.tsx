import '../styles.css'
import '@sentio/ui-core/dist/style.css'
import { NewSimulation } from './NewSimulation'
import type { SimulationFormType, Contract, AbiFunction } from './types'

// Mock contract data
const mockContract: Contract = {
  address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
  chainId: '1',
  name: 'USDC',
  abi: [
    {
      name: 'transfer',
      type: 'function',
      inputs: [
        {
          name: 'recipient',
          type: 'address',
          internalType: 'address'
        },
        {
          name: 'amount',
          type: 'uint256',
          internalType: 'uint256'
        }
      ],
      outputs: [
        {
          name: '',
          type: 'bool',
          internalType: 'bool'
        }
      ],
      stateMutability: 'nonpayable'
    },
    {
      name: 'approve',
      type: 'function',
      inputs: [
        {
          name: 'spender',
          type: 'address',
          internalType: 'address'
        },
        {
          name: 'amount',
          type: 'uint256',
          internalType: 'uint256'
        }
      ],
      outputs: [
        {
          name: '',
          type: 'bool',
          internalType: 'bool'
        }
      ],
      stateMutability: 'nonpayable'
    },
    {
      name: 'balanceOf',
      type: 'function',
      inputs: [
        {
          name: 'account',
          type: 'address',
          internalType: 'address'
        }
      ],
      outputs: [
        {
          name: '',
          type: 'uint256',
          internalType: 'uint256'
        }
      ],
      stateMutability: 'view'
    }
  ]
}

const mockDefaultValue: Partial<SimulationFormType> = {
  from: '0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb',
  blockNumber: 18500000,
  contract: mockContract,
  function: mockContract.abi?.[0]
}

export const Default = () => {
  return (
    <div className="h-screen w-full bg-gray-50 p-4">
      <div className="mx-auto max-w-2xl rounded-lg bg-white shadow-lg">
        <NewSimulation
          onSuccess={(result) => {
            console.log('Simulation success:', result)
            alert('Simulation submitted successfully!')
          }}
        />
      </div>
    </div>
  )
}

export const WithDefaultValues = () => {
  return (
    <div className="h-screen w-full bg-gray-50 p-4">
      <div className="mx-auto max-w-2xl rounded-lg bg-white shadow-lg">
        <NewSimulation
          defaultValue={mockDefaultValue}
          onSuccess={(result) => {
            console.log('Simulation success:', result)
          }}
        />
      </div>
    </div>
  )
}

export const WithContractPreselected = () => {
  return (
    <div className="h-screen w-full bg-gray-50 p-4">
      <div className="mx-auto max-w-2xl rounded-lg bg-white shadow-lg">
        <NewSimulation
          defaultValue={{
            from: '0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045',
            contract: {
              address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
              chainId: '1',
              name: 'WETH',
              abi: [
                {
                  name: 'deposit',
                  type: 'function',
                  inputs: [],
                  outputs: [],
                  stateMutability: 'payable'
                },
                {
                  name: 'withdraw',
                  type: 'function',
                  inputs: [
                    {
                      name: 'wad',
                      type: 'uint256',
                      internalType: 'uint256'
                    }
                  ],
                  outputs: [],
                  stateMutability: 'nonpayable'
                }
              ]
            }
          }}
          onSuccess={(result) => {
            console.log('Simulation success:', result)
          }}
        />
      </div>
    </div>
  )
}

export const WithStateOverrides = () => {
  return (
    <div className="h-screen w-full bg-gray-50 p-4">
      <div className="mx-auto max-w-2xl rounded-lg bg-white shadow-lg">
        <NewSimulation
          defaultValue={{
            ...mockDefaultValue,
            stateOverride: [
              {
                contract: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
                balance: '1000000000000000000',
                storage: [
                  {
                    key: '0x0',
                    value: '0x1'
                  }
                ]
              }
            ]
          }}
          onSuccess={(result) => {
            console.log('Simulation success:', result)
          }}
        />
      </div>
    </div>
  )
}

export const WithBlockOverrides = () => {
  return (
    <div className="h-screen w-full bg-gray-50 p-4">
      <div className="mx-auto max-w-2xl rounded-lg bg-white shadow-lg">
        <NewSimulation
          defaultValue={{
            ...mockDefaultValue,
            header: {
              blockNumber: 18500100,
              blockNumberState: true,
              timestamp: Date.now(),
              timestampState: true
            }
          }}
          onSuccess={(result) => {
            console.log('Simulation success:', result)
          }}
        />
      </div>
    </div>
  )
}

export const WithRawCallData = () => {
  return (
    <div className="h-screen w-full bg-gray-50 p-4">
      <div className="mx-auto max-w-2xl rounded-lg bg-white shadow-lg">
        <NewSimulation
          defaultValue={{
            from: '0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb',
            contract: {
              address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
              chainId: '1',
              name: 'USDC'
            },
            input:
              '0xa9059cbb000000000000000000000000742d35cc6634c0532925a3b844bc9e7595f0beb0000000000000000000000000000000000000000000000000000000000000064'
          }}
          onSuccess={(result) => {
            console.log('Simulation success:', result)
          }}
        />
      </div>
    </div>
  )
}

export const WithCustomCallback = () => {
  return (
    <div className="h-screen w-full bg-gray-50 p-4">
      <div className="mx-auto max-w-2xl rounded-lg bg-white shadow-lg">
        <NewSimulation
          defaultValue={mockDefaultValue}
          onChange={(formData, atomState) => {
            console.log('Form changed:', formData, atomState)
          }}
          onSuccess={(result) => {
            console.log('Simulation success:', result)
            alert(`Simulation submitted! ID: ${result.simulation?.id}`)
          }}
          onClose={() => {
            console.log('Close requested')
          }}
        />
      </div>
    </div>
  )
}

export const WithCustomRequestAPI = () => {
  const mockAPIRequest = async (request: any) => {
    // Simulate API call
    console.log('Mock API Request:', request)
    await new Promise((resolve) => setTimeout(resolve, 2000))

    return {
      simulation: {
        id: 'mock-simulation-' + Date.now(),
        status: 'pending'
      }
    }
  }

  return (
    <div className="h-screen w-full bg-gray-50 p-4">
      <div className="mx-auto max-w-2xl rounded-lg bg-white shadow-lg">
        <NewSimulation
          defaultValue={mockDefaultValue}
          onRequestAPI={mockAPIRequest}
          onSuccess={(result) => {
            console.log('Simulation success:', result)
            alert(`Mock simulation created! ID: ${result.simulation?.id}`)
          }}
        />
      </div>
    </div>
  )
}

export const CompactMode = () => {
  return (
    <div className="h-screen w-full bg-gray-50 p-4">
      <div className="mx-auto max-w-lg rounded-lg bg-white shadow-lg">
        <NewSimulation
          defaultValue={{
            from: '0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb',
            contract: mockContract
          }}
          hideProjectSelect={true}
          hideNetworkSelect={true}
          onSuccess={(result) => {
            console.log('Simulation success:', result)
          }}
        />
      </div>
    </div>
  )
}
