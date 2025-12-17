import '../styles.css'
import { TransactionStatus, TransactionValue, AddressFrom, AddressTo, TransactionLabel } from './TransactionComponents'

// Ladle works by rendering exported React components from *.stories.* files.
// Export simple React components (no Storybook APIs) so Ladle can pick them up.

export const transactionStatus = () => (
  <div className="p-4 text-sm flex gap-4 items-center">
    <TransactionStatus status={1} />
    <TransactionStatus status={0} />
    <TransactionStatus status={2} />
  </div>
)

export const TransactionValueWithChainId = () => (
  <div className="p-4 text-sm">
    <TransactionValue value="5000000000000000000" chainId="1" />
  </div>
)

export const AddressExample = () => (
  <div className="p-4 text-sm">
    <AddressFrom address="0x742d35Cc6634C0532925a3b844Bc454e4438f44e" />
    <AddressTo address="0x8ba1f109551bd432803012645ac136ddd64dba72e" />
  </div>
)

export const TransactionLabelInternal = () => (
  <div className="p-4 text-sm flex flex-col gap-4">
    <h1 className="text-base">
      Transaction Label - Internal Transaction
    </h1>
    <TransactionLabel
      row={{ original: { trace: true } }}
      getValue={() => "0x8ba1f109551bd432803012645ac136ddd64dba72e3a22cb58b4f2e0a7d4b3c5f1"}
    />
    <h1 className="text-base">
      Transaction Label - External Transaction
    </h1>
    <TransactionLabel
      row={{ original: { trace: false } }}
      getValue={() => "0x1234567890abcdef"}
    />
  </div>
)
