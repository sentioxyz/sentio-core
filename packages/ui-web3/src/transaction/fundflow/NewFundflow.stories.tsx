import '../../styles.css'
import '@sentio/ui-core/dist/style.css'
import { TransactionFundflow } from './NewFundflow'
import { DecodedCallTrace } from '@sentio/debugger-common'
import { Transaction } from '../types'
import { useState } from 'react'
import { SvgFolderContext } from '@sentio/ui-core'

// Mock transaction data
const createMockTransaction = (): Transaction =>
  ({
    hash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
    from: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
    to: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D',
    value: '0x0',
    gasUsed: '0x15f90',
    status: 1,
    blockNumber: '12345678'
  }) as any

// Simple token transfer call trace
const createSimpleTransferTrace = (): DecodedCallTrace =>
  ({
    type: 'CALL',
    from: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
    to: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984',
    value: '0x0',
    gasUsed: '0x5208',
    input: '0xa9059cbb',
    output:
      '0x0000000000000000000000000000000000000000000000000000000000000001',
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
    calls: [],
    logs: [
      {
        address: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984',
        topics: [
          '0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef',
          '0x000000000000000000000000742d35cc6634c0532925a3b844bc454e4438f44e',
          '0x0000000000000000000000008ba1f109551bd432803012645ac136ddd64dba72'
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
            value: '0x8ba1f109551bd432803012645ac136ddd64dba72'
          },
          {
            name: 'value',
            type: 'uint256',
            value: '1000000000000000000'
          }
        ],
        location: {
          compilationId: 'contract_1',
          instructionIndex: 5
        }
      }
    ],
    location: {
      compilationId: 'contract_1',
      instructionIndex: 0
    }
  }) as any

// Complex multi-hop swap trace
const createMultiHopSwapTrace = (): DecodedCallTrace =>
  ({
    type: 'CALL',
    from: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
    to: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D',
    value: '0x0',
    gasUsed: '0x25f90',
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
          '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984', // UNI
          '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2', // WETH
          '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48' // USDC
        ]
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
              compilationId: 'contract_1',
              instructionIndex: 10
            }
          }
        ],
        location: {
          compilationId: 'contract_1',
          instructionIndex: 5
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
        calls: [
          {
            type: 'CALL',
            from: '0xd3d2E2692501A5c9Ca623199D38826e513033a17',
            to: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
            value: '0x0',
            contractName: 'WETH',
            functionName: 'transfer(address,uint256)',
            calls: [],
            logs: [
              {
                address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
                topics: [
                  '0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef',
                  '0x000000000000000000000000d3d2e2692501a5c9ca623199d38826e513033a17',
                  '0x000000000000000000000000b4e16d0168e52d35cacd2c6185b44281ec28c9dc'
                ],
                data: '0x00000000000000000000000000000000000000000000000029a2241af62c0000',
                name: 'Transfer(address,address,uint256)',
                events: [
                  {
                    name: 'from',
                    type: 'address',
                    value: '0xd3d2E2692501A5c9Ca623199D38826e513033a17'
                  },
                  {
                    name: 'to',
                    type: 'address',
                    value: '0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc'
                  },
                  {
                    name: 'value',
                    type: 'uint256',
                    value: '3000000000000000000'
                  }
                ],
                location: {
                  compilationId: 'contract_2',
                  instructionIndex: 20
                }
              }
            ],
            location: {
              compilationId: 'contract_2',
              instructionIndex: 15
            }
          }
        ],
        logs: [],
        location: {
          compilationId: 'contract_1',
          instructionIndex: 15
        }
      },
      {
        type: 'CALL',
        from: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D',
        to: '0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc',
        value: '0x0',
        contractName: 'UniswapV2Pair',
        functionName: 'swap(uint256,uint256,address,bytes)',
        calls: [
          {
            type: 'CALL',
            from: '0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc',
            to: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
            value: '0x0',
            contractName: 'USDC',
            functionName: 'transfer(address,uint256)',
            calls: [],
            logs: [
              {
                address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
                topics: [
                  '0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef',
                  '0x000000000000000000000000b4e16d0168e52d35cacd2c6185b44281ec28c9dc',
                  '0x000000000000000000000000742d35cc6634c0532925a3b844bc454e4438f44e'
                ],
                data: '0x0000000000000000000000000000000000000000000000000000000077359400',
                name: 'Transfer(address,address,uint256)',
                events: [
                  {
                    name: 'from',
                    type: 'address',
                    value: '0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc'
                  },
                  {
                    name: 'to',
                    type: 'address',
                    value: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e'
                  },
                  {
                    name: 'value',
                    type: 'uint256',
                    value: '2000000000' // 2000 USDC (6 decimals)
                  }
                ],
                location: {
                  compilationId: 'contract_3',
                  instructionIndex: 30
                }
              }
            ],
            location: {
              compilationId: 'contract_3',
              instructionIndex: 25
            }
          }
        ],
        logs: [],
        location: {
          compilationId: 'contract_1',
          instructionIndex: 25
        }
      }
    ],
    logs: [],
    location: {
      compilationId: 'contract_1',
      instructionIndex: 0
    }
  }) as any

// ETH transfer trace
const createETHTransferTrace = (): DecodedCallTrace =>
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

// Empty trace (no transfers)
const createEmptyTrace = (): DecodedCallTrace =>
  ({
    type: 'CALL',
    from: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
    to: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984',
    value: '0x0',
    gasUsed: '0x5208',
    contractName: 'Contract',
    functionName: 'someFunction()',
    inputs: [],
    calls: [],
    logs: [],
    location: {
      compilationId: 'contract_1',
      instructionIndex: 0
    }
  }) as any

// Basic fund flow
export const Default = () => {
  return (
    <div className="p-4">
      <h2 className="mb-4 text-lg font-semibold">Simple Token Transfer</h2>
      <div className="rounded border">
        <TransactionFundflow
          transaction={createMockTransaction()}
          data={createSimpleTransferTrace()}
          chainId="1"
        />
      </div>
    </div>
  )
}

// Multi-hop swap fund flow
export const MultiHopSwap = () => {
  return (
    <div className="p-4">
      <h2 className="mb-4 text-lg font-semibold">
        Multi-Hop Token Swap (UNI → WETH → USDC)
      </h2>
      <div className="rounded border">
        <TransactionFundflow
          transaction={createMockTransaction()}
          data={createMultiHopSwapTrace()}
          chainId="1"
        />
      </div>
    </div>
  )
}

// ETH deposit
export const ETHDeposit = () => {
  return (
    <div className="p-4">
      <h2 className="mb-4 text-lg font-semibold">ETH Deposit to WETH</h2>
      <div className="rounded border">
        <TransactionFundflow
          transaction={createMockTransaction()}
          data={createETHTransferTrace()}
          chainId="1"
        />
      </div>
    </div>
  )
}

// Empty fund flow
export const EmptyFundFlow = () => {
  return (
    <SvgFolderContext.Provider value="https://app.sentio.xyz">
      <div className="p-4">
        <h2 className="mb-4 text-lg font-semibold">
          Transaction with No Fund Flow
        </h2>
        <div className="rounded border">
          <TransactionFundflow
            transaction={createMockTransaction()}
            data={createEmptyTrace()}
            chainId="1"
          />
        </div>
      </div>
    </SvgFolderContext.Provider>
  )
}

// Custom empty component
export const CustomEmptyState = () => {
  return (
    <div className="p-4">
      <h2 className="mb-4 text-lg font-semibold">Custom Empty State</h2>
      <div className="rounded border">
        <TransactionFundflow
          transaction={createMockTransaction()}
          data={createEmptyTrace()}
          chainId="1"
          empty={
            <div className="flex h-[400px] items-center justify-center">
              <div className="text-center">
                <p className="text-lg font-semibold text-gray-600">
                  No Transfers Found
                </p>
                <p className="text-sm text-gray-500">
                  This transaction did not transfer any tokens
                </p>
              </div>
            </div>
          }
        />
      </div>
    </div>
  )
}

// With loading state
export const LoadingState = () => {
  return (
    <SvgFolderContext.Provider value="https://app.sentio.xyz">
      <div className="p-4">
        <h2 className="mb-4 text-lg font-semibold">Loading State</h2>
        <div className="rounded border">
          <TransactionFundflow
            transaction={createMockTransaction()}
            data={createSimpleTransferTrace()}
            dataLoading={true}
            chainId="1"
          />
        </div>
      </div>
    </SvgFolderContext.Provider>
  )
}

// With onEmpty callback
export const WithEmptyCallback = () => {
  const [isEmpty, setIsEmpty] = useState(false)

  return (
    <SvgFolderContext.Provider value="https://app.sentio.xyz">
      <div className="p-4">
        <h2 className="mb-4 text-lg font-semibold">With Empty Callback</h2>
        <div className="mb-2 rounded bg-blue-50 p-2 text-sm">
          <strong>Is Empty:</strong> {isEmpty ? 'Yes' : 'No'}
        </div>
        <div className="rounded border">
          <TransactionFundflow
            transaction={createMockTransaction()}
            data={createEmptyTrace()}
            chainId="1"
            onEmpty={(empty) => {
              console.log('Fund flow empty:', empty)
              setIsEmpty(empty)
            }}
          />
        </div>
      </div>
    </SvgFolderContext.Provider>
  )
}

// Different chain (Polygon)
export const PolygonChain = () => {
  return (
    <div className="p-4">
      <h2 className="mb-4 text-lg font-semibold">
        Fund Flow on Polygon (Chain ID: 137)
      </h2>
      <div className="rounded border">
        <TransactionFundflow
          transaction={{
            ...createMockTransaction(),
            hash: '0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890'
          }}
          data={createSimpleTransferTrace()}
          chainId="137"
        />
      </div>
    </div>
  )
}

// With tag map
export const WithTagMap = () => {
  const tagMap = new Map([
    [
      '0x742d35cc6634c0532925a3b844bc454e4438f44e',
      { name: 'My Wallet', tag: 'personal' }
    ],
    [
      '0x8ba1f109551bd432803012645ac136ddd64dba72',
      { name: 'Exchange Wallet', tag: 'exchange' }
    ],
    [
      '0x1f9840a85d5af5bf1d1762f925bdaddc4201f984',
      { name: 'UNI Token', tag: 'token' }
    ]
  ])

  const defaultTagMap = new Map([
    ['0x742d35cc6634c0532925a3b844bc454e4438f44e', 'Sender'],
    ['0x8ba1f109551bd432803012645ac136ddd64dba72', 'Receiver']
  ])

  return (
    <div className="p-4">
      <h2 className="mb-4 text-lg font-semibold">
        Fund Flow with Address Tags
      </h2>
      <div className="rounded border">
        <TransactionFundflow
          transaction={createMockTransaction()}
          data={createSimpleTransferTrace()}
          chainId="1"
          tagMap={tagMap}
          defaultTagMap={defaultTagMap}
        />
      </div>
    </div>
  )
}

// With setTagAddressList callback
export const WithTagAddressList = () => {
  const [addresses, setAddresses] = useState<string[]>([])

  return (
    <div className="p-4">
      <h2 className="mb-4 text-lg font-semibold">
        Fund Flow with Tag Address List Callback
      </h2>
      <div className="mb-2 rounded bg-blue-50 p-2 text-sm">
        <strong>Tagged Addresses:</strong>
        <ul className="ml-4 mt-1 list-disc">
          {addresses.length > 0 ? (
            addresses.map((addr, idx) => <li key={idx}>{addr}</li>)
          ) : (
            <li>None</li>
          )}
        </ul>
      </div>
      <div className="rounded border">
        <TransactionFundflow
          transaction={createMockTransaction()}
          data={createSimpleTransferTrace()}
          chainId="1"
          setTagAddressList={(addrs) => {
            console.log('Tag addresses:', addrs)
            setAddresses(addrs)
          }}
        />
      </div>
    </div>
  )
}

// Complex multi-transfer scenario
export const ComplexScenario = () => {
  return (
    <div className="p-4">
      <h2 className="mb-4 text-lg font-semibold">
        Complex Multi-Transfer Scenario
      </h2>
      <p className="mb-4 text-sm text-gray-600">
        Uniswap swap with multiple hops showing the complete token flow path
      </p>
      <div className="rounded border">
        <TransactionFundflow
          transaction={createMockTransaction()}
          data={createMultiHopSwapTrace()}
          chainId="1"
          tagMap={
            new Map([
              [
                '0x742d35cc6634c0532925a3b844bc454e4438f44e',
                { name: 'Trader', tag: 'user' }
              ],
              [
                '0x7a250d5630b4cf539739df2c5dacb4c659f2488d',
                { name: 'Uniswap Router', tag: 'contract' }
              ],
              [
                '0xd3d2e2692501a5c9ca623199d38826e513033a17',
                { name: 'UNI/WETH Pool', tag: 'pool' }
              ],
              [
                '0xb4e16d0168e52d35cacd2c6185b44281ec28c9dc',
                { name: 'WETH/USDC Pool', tag: 'pool' }
              ]
            ])
          }
        />
      </div>
    </div>
  )
}
