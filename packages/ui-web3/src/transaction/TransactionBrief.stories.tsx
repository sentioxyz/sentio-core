import '../styles.css'
import '@sentio/ui-core/dist/style.css'
import { TransactionBrief } from './TransactionBrief'
import { Transaction, Block, TransactionReciept } from './types'
import { SvgFolderContext } from '@sentio/ui-core'

// Mock transaction data
const createMockTransaction = (overrides?: Partial<Transaction>): Transaction =>
  ({
    hash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
    from: '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
    to: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D',
    value: '0x0',
    gas: '0x30d40', // 200,000
    gasPrice: '0x4a817c800', // 20 Gwei
    nonce: '0x1a',
    input: '0x38ed1739',
    transactionIndex: '0x5',
    type: '0x2', // EIP-1559
    maxFeePerGas: '0x5d21dba00', // 25 Gwei
    maxPriorityFeePerGas: '0x3b9aca00', // 1 Gwei
    ...overrides
  }) as any

const createMockBlock = (overrides?: Partial<Block>): Block =>
  ({
    number: '0xbc614e', // 12345678
    timestamp: '0x639a8c40', // Dec 14, 2022
    baseFeePerGas: '0x4a817c800', // 20 Gwei
    ...overrides
  }) as any

const createMockReceipt = (
  overrides?: Partial<TransactionReciept>
): TransactionReciept =>
  ({
    status: '0x1', // Success
    gasUsed: '0x2a300', // 172,800
    ...overrides
  }) as any

// Wrapper component for all stories
const StoryWrapper = ({ children }: { children: React.ReactNode }) => (
  <SvgFolderContext.Provider value="https://app.sentio.xyz">
    {children}
  </SvgFolderContext.Provider>
)

// Successful transaction
export const SuccessfulTransaction = () => {
  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">Successful Transaction</h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction()}
            block={createMockBlock()}
            receipt={createMockReceipt()}
            latestBlockNumber="0xbc6164" // 12345700 (22 blocks later)
            chainId="1"
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

// Failed transaction
export const FailedTransaction = () => {
  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">Failed Transaction</h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction()}
            block={createMockBlock()}
            receipt={createMockReceipt({ status: '0x0' })}
            latestBlockNumber="0xbc6164"
            chainId="1"
            error="revert"
            errorReason={
              <span className="ml-2 text-red-600">Insufficient balance</span>
            }
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

// Pending transaction (no block/receipt yet)
export const PendingTransaction = () => {
  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">Pending Transaction</h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction()}
            block={{} as Block}
            receipt={{} as TransactionReciept}
            chainId="1"
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

// Legacy transaction (Type 0)
export const LegacyTransaction = () => {
  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">
          Legacy Transaction (EIP-2718)
        </h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction({
              type: '0x0',
              maxFeePerGas: undefined,
              maxPriorityFeePerGas: undefined
            })}
            block={createMockBlock({ baseFeePerGas: undefined })}
            receipt={createMockReceipt()}
            latestBlockNumber="0xbc6164"
            chainId="1"
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

// EIP-1559 transaction with max fees
export const EIP1559Transaction = () => {
  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">EIP-1559 Transaction</h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction({
              type: '0x2',
              maxFeePerGas: '0x77359400', // 2,000,000,000 (2 Gwei)
              maxPriorityFeePerGas: '0x3b9aca00' // 1,000,000,000 (1 Gwei)
            })}
            block={createMockBlock({
              baseFeePerGas: '0x2540be400' // 10,000,000,000 (10 Gwei)
            })}
            receipt={createMockReceipt()}
            latestBlockNumber="0xbc6164"
            chainId="1"
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

// Contract creation transaction (no "to" address)
export const ContractCreation = () => {
  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">Contract Creation</h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction({ to: undefined })}
            block={createMockBlock()}
            receipt={createMockReceipt()}
            latestBlockNumber="0xbc6164"
            chainId="1"
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

// Transaction with gas refund
export const WithGasRefund = () => {
  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">
          Transaction with Gas Refund
        </h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction()}
            block={createMockBlock()}
            receipt={createMockReceipt()}
            latestBlockNumber="0xbc6164"
            chainId="1"
            refund="0x2710" // 10,000 gas refunded
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

// Polygon transaction
export const PolygonTransaction = () => {
  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">
          Polygon (MATIC) Transaction
        </h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction({
              gasPrice: '0x3b9aca00' // 1 Gwei (cheaper on Polygon)
            })}
            block={createMockBlock()}
            receipt={createMockReceipt({
              gasUsed: '0x186a0' // 100,000
            })}
            latestBlockNumber="0xbc6164"
            chainId="137"
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

// BSC transaction
export const BSCTransaction = () => {
  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">BSC (BNB) Transaction</h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction({
              gasPrice: '0x12a05f200' // 5 Gwei
            })}
            block={createMockBlock()}
            receipt={createMockReceipt()}
            latestBlockNumber="0xbc6164"
            chainId="56"
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

// High gas transaction
export const HighGasTransaction = () => {
  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">High Gas Transaction</h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction({
              gas: '0x7a120', // 500,000
              gasPrice: '0x3a35294400' // 250 Gwei
            })}
            block={createMockBlock({
              baseFeePerGas: '0x2e90edd000' // 200 Gwei
            })}
            receipt={createMockReceipt({
              gasUsed: '0x6ddd0' // 450,000
            })}
            latestBlockNumber="0xbc6164"
            chainId="1"
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

// Transaction with simulation info
export const WithSimulationInfo = () => {
  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">
          Transaction with Simulation Info
        </h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction()}
            block={createMockBlock()}
            receipt={createMockReceipt()}
            latestBlockNumber="0xbc6164"
            chainId="1"
            simulationId="sim-123456"
            renderSimulationInfo={(simId) => (
              <div className="mt-2 rounded bg-blue-50 p-2 text-sm dark:bg-blue-900/20">
                <span className="font-semibold">Simulation:</span> {simId}
                <button className="ml-2 text-blue-600 underline hover:text-blue-700">
                  View Original Transaction
                </button>
              </div>
            )}
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

// Recent transaction (just now)
export const RecentTransaction = () => {
  const now = Math.floor(Date.now() / 1000)
  const recentTimestamp = `0x${(now - 30).toString(16)}` // 30 seconds ago

  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">
          Recent Transaction (30s ago)
        </h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction()}
            block={createMockBlock({ timestamp: recentTimestamp })}
            receipt={createMockReceipt()}
            latestBlockNumber="0xbc6164"
            chainId="1"
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

// Old transaction
export const OldTransaction = () => {
  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">Old Transaction (2020)</h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction()}
            block={createMockBlock({
              number: '0x989680', // 10,000,000
              timestamp: '0x5f5e1000' // Sep 13, 2020
            })}
            receipt={createMockReceipt()}
            latestBlockNumber="0xbc6164"
            chainId="1"
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

// Multiple transactions comparison
export const TransactionComparison = () => {
  return (
    <StoryWrapper>
      <div className="space-y-6 p-4 text-sm">
        <h2 className="text-lg font-semibold">Transaction Comparison</h2>

        <div>
          <h3 className="text-md mb-2 font-medium text-gray-700 dark:text-gray-300">
            Low Gas Price (Fast)
          </h3>
          <div className="rounded border bg-white p-4 dark:bg-gray-900">
            <TransactionBrief
              transaction={createMockTransaction({ gasPrice: '0x9502f900' })} // 2.5 Gwei
              block={createMockBlock()}
              receipt={createMockReceipt()}
              chainId="1"
            />
          </div>
        </div>

        <div>
          <h3 className="text-md mb-2 font-medium text-gray-700 dark:text-gray-300">
            Medium Gas Price (Standard)
          </h3>
          <div className="rounded border bg-white p-4 dark:bg-gray-900">
            <TransactionBrief
              transaction={createMockTransaction({ gasPrice: '0x4a817c800' })} // 20 Gwei
              block={createMockBlock()}
              receipt={createMockReceipt()}
              chainId="1"
            />
          </div>
        </div>

        <div>
          <h3 className="text-md mb-2 font-medium text-gray-700 dark:text-gray-300">
            High Gas Price (Priority)
          </h3>
          <div className="rounded border bg-white p-4 dark:bg-gray-900">
            <TransactionBrief
              transaction={createMockTransaction({ gasPrice: '0x37e11d600' })} // 150 Gwei
              block={createMockBlock()}
              receipt={createMockReceipt()}
              chainId="1"
            />
          </div>
        </div>
      </div>
    </StoryWrapper>
  )
}

// Complex error case
export const ComplexErrorCase = () => {
  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">Complex Error Case</h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction()}
            block={createMockBlock()}
            receipt={createMockReceipt({ status: '0x0' })}
            latestBlockNumber="0xbc6164"
            chainId="1"
            error="execution reverted"
            errorReason={
              <div className="ml-2">
                <div className="font-mono text-sm text-red-600">
                  Error: VM Exception while processing transaction: revert
                </div>
                <div className="mt-1 text-xs text-red-500">
                  Reason: ERC20: transfer amount exceeds balance
                </div>
              </div>
            }
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

// Minimal data (edge case)
export const MinimalData = () => {
  return (
    <StoryWrapper>
      <div className="p-4 text-sm">
        <h2 className="mb-4 text-lg font-semibold">Minimal Data (Edge Case)</h2>
        <div className="rounded border bg-white p-4 dark:bg-gray-900">
          <TransactionBrief
            transaction={createMockTransaction({
              gasPrice: '0x0',
              nonce: '0x0',
              transactionIndex: '0x0'
            })}
            block={{} as Block}
            receipt={{} as TransactionReciept}
            chainId="1"
          />
        </div>
      </div>
    </StoryWrapper>
  )
}
