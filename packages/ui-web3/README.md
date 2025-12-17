# @sentio/ui-web3

A Web3-focused UI component library for displaying blockchain transactions and address information.

## Features

- 🔗 Full set of Web3 transaction components
- 💰 Display balance changes
- 🏷️ Address labels and name resolution
- 📊 Chain data visualization
- ♻️ Reuses styles and components from `@sentio/ui-core`

## Installation

```bash
pnpm add @sentio/ui-web3 @sentio/ui-core
```

Note: This package depends on `@sentio/ui-core` and re-exports all of its contents, so you can import everything from `@sentio/ui-web3` if preferred.

## Usage

```tsx
// Includes both ui-core and ui-web3 components
import { 
  Button,           // from ui-core
  BarLoading,       // from ui-core
  BalanceChanges,   // from ui-web3
  HexNumber,        // from ui-web3
  TransactionStatus // from ui-web3
} from '@sentio/ui-web3'

import '@sentio/ui-core/dist/style.css'

function TransactionView({ transaction, block }) {
  return (
    <div>
      <TransactionStatus status={transaction.status} />
      <BalanceChanges transaction={transaction} block={block} />
      <HexNumber data={transaction.hash} />
    </div>
  )
}
```

## Web3 components

### Transaction components
- `BalanceChanges` - Display balance changes
- `HexNumber` - Hex number / address display (with link and copy)
- `TransactionStatus` - Transaction status display
- `TransactionValue` - Transaction value display
- `TransactionLabel` - Transaction label
- `AddressFrom` / `AddressTo` - Address display

### Utility Hooks
- `useAddressTag()` - Fetch address tag information
- `usePrice()` - Fetch token price
- `useFallbackName()` - Get fallback contract name

## Using ui-core alone

If your project doesn't need Web3 functionality, you can install and use just `@sentio/ui-core`:

```bash
pnpm add @sentio/ui-core
```

```tsx
import { Button, BaseDialog, BarLoading } from '@sentio/ui-core'
import '@sentio/ui-core/dist/style.css'
```

## Architecture

```
@sentio/ui-web3
    ↓ depends on & re-exports
@sentio/ui-core
    ↓ provides
Core components + styles
```

Users can:
1. Install only `@sentio/ui-core` — get core UI components.
2. Install `@sentio/ui-web3` — automatically includes `ui-core` plus Web3 components.
