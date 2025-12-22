import '../../styles.css'
import '@sentio/ui-core/dist/style.css'
import { FlatCallTraceTree } from './FlatCallTrace'
import { DecodedCallTrace } from '@sentio/debugger-common'
import {
  ChainIdContext,
  OverviewContext,
  GlobalQueryContext
} from '../transaction-context'
import { useState } from 'react'
import { SvgFolderContext } from '@sentio/ui-core'

// Sample call trace data
const createSimpleCallTrace = (): DecodedCallTrace =>
  ({
    type: 'CALL',
    from: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
    to: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984',
    value: '0x0',
    gasUsed: '0x5208',
    input:
      '0xa9059cbb0000000000000000000000008ba1f109551bd432803012645ac136ddd64dba720000000000000000000000000000000000000000000000000de0b6b3a7640000',
    output:
      '0x0000000000000000000000000000000000000000000000000000000000000001',
    calls: [],
    logs: [],
    error: undefined,
    contractName: 'UNI',
    functionName: 'transfer(address,uint256)',
    inputs: [
      {
        name: 'recipient',
        type: 'address',
        value: '0x8ba1f109551bd432803012645ac136ddd64dba72'
      },
      {
        name: 'amount',
        type: 'uint256',
        value: '1000000000000000000'
      }
    ],
    returnValue: [
      {
        name: '',
        type: 'bool',
        value: true
      }
    ],
    location: {
      compilationId: 'contract_1',
      instructionIndex: 0
    }
  }) as any

const createNestedCallTrace = (): DecodedCallTrace =>
  ({
    type: 'CALL',
    from: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
    to: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D',
    value: '0x0',
    gasUsed: '0x15f90',
    input: '0x38ed1739',
    output: '0x',
    contractName: 'UniswapV2Router02',
    functionName:
      'swapExactTokensForTokens(uint256,uint256,address[],address,uint256)',
    inputs: [
      {
        name: 'amountIn',
        type: 'uint256',
        value: '1000000000000000000'
      },
      {
        name: 'amountOutMin',
        type: 'uint256',
        value: '0'
      },
      {
        name: 'path',
        type: 'address[]',
        value: [
          '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984',
          '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2'
        ]
      },
      {
        name: 'to',
        type: 'address',
        value: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e'
      },
      {
        name: 'deadline',
        type: 'uint256',
        value: '1700000000'
      }
    ],
    calls: [
      {
        type: 'CALL',
        from: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D',
        to: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984',
        value: '0x0',
        gasUsed: '0x5208',
        contractName: 'UNI',
        functionName: 'transferFrom(address,address,uint256)',
        inputs: [
          {
            name: 'sender',
            type: 'address',
            value: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e'
          },
          {
            name: 'recipient',
            type: 'address',
            value: '0xd3d2E2692501A5c9Ca623199D38826e513033a17'
          },
          {
            name: 'amount',
            type: 'uint256',
            value: '1000000000000000000'
          }
        ],
        returnValue: [
          {
            name: '',
            type: 'bool',
            value: true
          }
        ],
        calls: [],
        logs: [
          {
            address: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984',
            topics: [
              '0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef',
              '0x000000000000000000000000742d35cc6634c0532925a3b844bc454e4438f44e',
              '0x000000000000000000000000d3d2e2692501a5c9ca623199d38826e513033a17'
            ],
            data: '0x0000000000000000000000000000000000000000000000000de0b6b3a7640000',
            name: 'Transfer(address,address,uint256)',
            events: [
              {
                name: 'from',
                type: 'address',
                value: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e'
              },
              {
                name: 'to',
                type: 'address',
                value: '0xd3d2E2692501A5c9Ca623199D38826e513033a17'
              },
              {
                name: 'value',
                type: 'uint256',
                value: '1000000000000000000'
              }
            ],
            location: {
              compilationId: 'contract_2',
              instructionIndex: 15
            }
          }
        ],
        location: {
          compilationId: 'contract_2',
          instructionIndex: 10
        }
      },
      {
        type: 'CALL',
        from: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D',
        to: '0xd3d2E2692501A5c9Ca623199D38826e513033a17',
        value: '0x0',
        gasUsed: '0x7530',
        contractName: 'UniswapV2Pair',
        functionName: 'swap(uint256,uint256,address,bytes)',
        inputs: [
          {
            name: 'amount0Out',
            type: 'uint256',
            value: '0'
          },
          {
            name: 'amount1Out',
            type: 'uint256',
            value: '3000000000000000000'
          },
          {
            name: 'to',
            type: 'address',
            value: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e'
          },
          {
            name: 'data',
            type: 'bytes',
            value: '0x'
          }
        ],
        calls: [],
        logs: [
          {
            address: '0xd3d2E2692501A5c9Ca623199D38826e513033a17',
            topics: [
              '0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822'
            ],
            data: '0x0000000000000000000000007a250d5630b4cf539739df2c5dacb4c659f2488d0000000000000000000000000000000000000000000000000de0b6b3a76400000000000000000000000000000000000000000000000000000000000000000000',
            name: 'Swap(address,uint256,uint256,uint256,uint256,address)',
            events: [
              {
                name: 'sender',
                type: 'address',
                value: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D'
              },
              {
                name: 'amount0In',
                type: 'uint256',
                value: '1000000000000000000'
              },
              {
                name: 'amount1In',
                type: 'uint256',
                value: '0'
              },
              {
                name: 'amount0Out',
                type: 'uint256',
                value: '0'
              },
              {
                name: 'amount1Out',
                type: 'uint256',
                value: '3000000000000000000'
              },
              {
                name: 'to',
                type: 'address',
                value: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e'
              }
            ],
            location: {
              compilationId: 'contract_3',
              instructionIndex: 25
            }
          }
        ],
        location: {
          compilationId: 'contract_3',
          instructionIndex: 20
        }
      }
    ],
    logs: [],
    returnValue: [
      {
        name: 'amounts',
        type: 'uint256[]',
        value: ['1000000000000000000', '3000000000000000000']
      }
    ],
    location: {
      compilationId: 'contract_1',
      instructionIndex: 0
    }
  }) as any

const createErrorCallTrace = (): DecodedCallTrace =>
  ({
    type: 'CALL',
    from: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
    to: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984',
    value: '0x0',
    gasUsed: '0x5208',
    input: '0xa9059cbb',
    output: '0x',
    error: 'revert',
    revertReason: 'Insufficient balance',
    contractName: 'UNI',
    functionName: 'transfer(address,uint256)',
    inputs: [
      {
        name: 'recipient',
        type: 'address',
        value: '0x8ba1f109551bd432803012645ac136ddd64dba72'
      },
      {
        name: 'amount',
        type: 'uint256',
        value: '1000000000000000000'
      }
    ],
    calls: [],
    logs: [],
    location: {
      compilationId: 'contract_1',
      instructionIndex: 0
    },
    decodedError: {
      name: 'InsufficientBalance',
      inputs: [
        {
          name: 'available',
          type: 'uint256',
          value: '500000000000000000'
        },
        {
          name: 'required',
          type: 'uint256',
          value: '1000000000000000000'
        }
      ]
    }
  }) as any

const createCallTraceWithValue = (): DecodedCallTrace =>
  ({
    type: 'CALL',
    from: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
    to: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
    value: '0xde0b6b3a7640000', // 1 ETH
    gasUsed: '0x5208',
    input: '0xd0e30db0',
    output: '0x',
    contractName: 'WETH',
    functionName: 'deposit()',
    inputs: [],
    calls: [],
    logs: [
      {
        address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
        topics: [
          '0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c',
          '0x000000000000000000000000742d35cc6634c0532925a3b844bc454e4438f44e'
        ],
        data: '0x0000000000000000000000000000000000000000000000000de0b6b3a7640000',
        name: 'Deposit(address,uint256)',
        events: [
          {
            name: 'dst',
            type: 'address',
            value: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e'
          },
          {
            name: 'wad',
            type: 'uint256',
            value: '1000000000000000000'
          }
        ],
        location: {
          compilationId: 'contract_weth',
          instructionIndex: 5
        }
      }
    ],
    location: {
      compilationId: 'contract_weth',
      instructionIndex: 0
    }
  }) as any

const createStorageCallTrace = (): DecodedCallTrace =>
  ({
    type: 'CALL',
    from: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
    to: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984',
    value: '0x0',
    gasUsed: '0x5208',
    input: '0xa9059cbb',
    output:
      '0x0000000000000000000000000000000000000000000000000000000000000001',
    contractName: 'Token',
    functionName: 'transfer(address,uint256)',
    inputs: [
      {
        name: 'to',
        type: 'address',
        value: '0x8ba1f109551bd432803012645ac136ddd64dba72'
      },
      {
        name: 'amount',
        type: 'uint256',
        value: '1000000000000000000'
      }
    ],
    returnValue: [
      {
        name: '',
        type: 'bool',
        value: true
      }
    ],
    calls: [],
    logs: [],
    storages: [
      {
        type: 'SSTORE',
        address: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984',
        slot: '0x0000000000000000000000000000000000000000000000000000000000000003',
        value:
          '0x00000000000000000000000000000000000000000000003635c9adc5dea00000',
        decodedVariable: {
          decoded: true,
          variableName: 'balances',
          location: {
            compilationId: 'contract_1',
            instructionIndex: 8
          }
        },
        location: {
          compilationId: 'contract_1',
          instructionIndex: 8
        }
      },
      {
        type: 'SSTORE',
        address: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984',
        slot: '0x0000000000000000000000000000000000000000000000000000000000000004',
        value:
          '0x0000000000000000000000000000000000000000000000000de0b6b3a7640000',
        decodedVariable: {
          decoded: true,
          variableName: 'totalSupply',
          location: {
            compilationId: 'contract_1',
            instructionIndex: 12
          }
        },
        location: {
          compilationId: 'contract_1',
          instructionIndex: 12
        }
      }
    ],
    location: {
      compilationId: 'contract_1',
      instructionIndex: 0
    }
  }) as any

// Mock context providers
const MockProviders = ({
  children,
  chainId = '1'
}: {
  children: React.ReactNode
  chainId?: string
}) => {
  const [selectedInstruction, setSelectedInstruction] = useState<string>()

  const overviewContext = {
    routeTo: (path?: string, dropBuild?: boolean, newWindow?: boolean) => {
      console.log('Navigate to:', path, dropBuild, newWindow)
    },
    setMask: (show: boolean) => {
      console.log('Set mask:', show)
    }
  }

  const globalQueryContext = {}

  return (
    <SvgFolderContext.Provider value="https://app.sentio.xyz">
      <ChainIdContext.Provider value={chainId}>
        <OverviewContext.Provider value={overviewContext as any}>
          <GlobalQueryContext.Provider value={globalQueryContext as any}>
            {children}
          </GlobalQueryContext.Provider>
        </OverviewContext.Provider>
      </ChainIdContext.Provider>
    </SvgFolderContext.Provider>
  )
}

// Basic call trace
export const Default = () => {
  return (
    <MockProviders>
      <div className="p-4">
        <h2 className="mb-4 text-lg font-semibold">Simple Token Transfer</h2>
        <div className="rounded border">
          <FlatCallTraceTree data={createSimpleCallTrace()} />
        </div>
      </div>
    </MockProviders>
  )
}

// Nested call trace with multiple levels
export const NestedCalls = () => {
  return (
    <MockProviders>
      <div className="p-4">
        <h2 className="mb-4 text-lg font-semibold">
          Uniswap Token Swap (Nested Calls)
        </h2>
        <div className="rounded border">
          <FlatCallTraceTree data={createNestedCallTrace()} />
        </div>
      </div>
    </MockProviders>
  )
}

// Call trace with error
export const WithError = () => {
  return (
    <MockProviders>
      <div className="p-4">
        <h2 className="mb-4 text-lg font-semibold">
          Failed Transaction with Revert
        </h2>
        <div className="rounded border">
          <FlatCallTraceTree data={createErrorCallTrace()} />
        </div>
      </div>
    </MockProviders>
  )
}

// Call trace with ETH value transfer
export const WithValue = () => {
  return (
    <MockProviders>
      <div className="p-4">
        <h2 className="mb-4 text-lg font-semibold">
          WETH Deposit (With ETH Value)
        </h2>
        <div className="rounded border">
          <FlatCallTraceTree data={createCallTraceWithValue()} gasUsed />
        </div>
      </div>
    </MockProviders>
  )
}

// Call trace with storage changes
export const WithStorage = () => {
  return (
    <MockProviders>
      <div className="p-4">
        <h2 className="mb-4 text-lg font-semibold">
          Transaction with Storage Changes
        </h2>
        <div className="rounded border">
          <FlatCallTraceTree data={createStorageCallTrace()} showStorage />
        </div>
      </div>
    </MockProviders>
  )
}

// Virtualized view
export const VirtualizedView = () => {
  return (
    <MockProviders>
      <div className="p-4">
        <h2 className="mb-4 text-lg font-semibold">
          Virtualized Call Trace (Fixed Height)
        </h2>
        <div className="rounded border">
          <FlatCallTraceTree
            data={createNestedCallTrace()}
            virtual
            height={400}
          />
        </div>
      </div>
    </MockProviders>
  )
}

// With gas usage
export const WithGasUsage = () => {
  return (
    <MockProviders>
      <div className="p-4">
        <h2 className="mb-4 text-lg font-semibold">
          Call Trace with Gas Information
        </h2>
        <div className="rounded border">
          <FlatCallTraceTree data={createNestedCallTrace()} gasUsed />
        </div>
      </div>
    </MockProviders>
  )
}

// With expander (depth control)
export const WithExpandDepth = () => {
  const [expander, setExpander] = useState(1)

  return (
    <MockProviders>
      <div className="p-4">
        <div className="mb-4 flex items-center gap-4">
          <h2 className="text-lg font-semibold">
            Call Trace with Expand Depth Control
          </h2>
          <div className="flex items-center gap-2">
            <label className="text-sm text-gray-600">Depth:</label>
            <select
              value={expander}
              onChange={(e) => setExpander(Number(e.target.value))}
              className="rounded border px-2 py-1"
            >
              <option value={0}>0 - Collapsed</option>
              <option value={1}>1 - First Level</option>
              <option value={2}>2 - Second Level</option>
              <option value={3}>3 - All Levels</option>
            </select>
          </div>
        </div>
        <div className="rounded border">
          <FlatCallTraceTree
            data={createNestedCallTrace()}
            expander={expander}
          />
        </div>
      </div>
    </MockProviders>
  )
}

// With instruction handler
export const WithInstructionHandler = () => {
  const [lastInstruction, setLastInstruction] = useState<string>('')

  return (
    <MockProviders>
      <div className="p-4">
        <h2 className="mb-4 text-lg font-semibold">
          Call Trace with Instruction Handler
        </h2>
        <div className="mb-2 rounded bg-blue-50 p-2 text-sm">
          <strong>Last Selected:</strong> {lastInstruction || 'None'}
        </div>
        <div className="rounded border">
          <FlatCallTraceTree
            data={createNestedCallTrace()}
            onInstruction={(key, location) => {
              setLastInstruction(
                `Key: ${key}, Index: ${location?.instructionIndex}`
              )
              console.log('Instruction selected:', key, location)
            }}
          />
        </div>
      </div>
    </MockProviders>
  )
}

// Different chain ID (Polygon)
export const DifferentChain = () => {
  return (
    <MockProviders chainId="137">
      <div className="p-4">
        <h2 className="mb-4 text-lg font-semibold">
          Call Trace on Polygon (Chain ID: 137)
        </h2>
        <div className="rounded border">
          <FlatCallTraceTree data={createCallTraceWithValue()} gasUsed />
        </div>
      </div>
    </MockProviders>
  )
}

// All features combined
export const AllFeatures = () => {
  const [expander, setExpander] = useState(2)

  return (
    <MockProviders>
      <div className="p-4">
        <div className="mb-4 flex items-center gap-4">
          <h2 className="text-lg font-semibold">All Features Combined</h2>
          <div className="flex items-center gap-2">
            <label className="text-sm text-gray-600">Depth:</label>
            <select
              value={expander}
              onChange={(e) => setExpander(Number(e.target.value))}
              className="rounded border px-2 py-1"
            >
              <option value={0}>0</option>
              <option value={1}>1</option>
              <option value={2}>2</option>
              <option value={3}>3</option>
            </select>
          </div>
        </div>
        <div className="rounded border">
          <FlatCallTraceTree
            data={createNestedCallTrace()}
            virtual
            height={500}
            gasUsed
            showStorage
            expander={expander}
            onInstruction={(key, location) => {
              console.log('Selected:', key, location)
            }}
          />
        </div>
      </div>
    </MockProviders>
  )
}
