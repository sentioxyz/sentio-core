# @sentio/ui-web3

Web3 专用 UI 组件库,用于显示区块链交易和地址信息。

## 特性

- 🔗 完整的 Web3 交易组件
- 💰 余额变化展示
- 🏷️ 地址标签和名称解析
- 📊 链数据可视化
- ♻️ 自动复用 @sentio/ui-core 的样式和组件

## 安装

```bash
pnpm add @sentio/ui-web3 @sentio/ui-core
```

注意:这个包依赖 `@sentio/ui-core`,并自动重新导出其所有内容,所以你可以只从 `@sentio/ui-web3` 导入所有组件。

## 使用

```tsx
// 同时包含 ui-core 和 ui-web3 的组件
import { 
  Button,           // 来自 ui-core
  BarLoading,       // 来自 ui-core
  BalanceChanges,   // 来自 ui-web3
  HexNumber,        // 来自 ui-web3
  TransactionStatus // 来自 ui-web3
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

## Web3 组件

### 交易组件
- `BalanceChanges` - 余额变化展示
- `HexNumber` - 十六进制数字/地址展示(带链接和复制功能)
- `TransactionStatus` - 交易状态展示
- `TransactionValue` - 交易金额展示
- `TransactionLabel` - 交易标签
- `AddressFrom` / `AddressTo` - 地址展示

### 工具 Hooks
- `useAddressTag()` - 获取地址标签信息
- `usePrice()` - 获取代币价格
- `useFallbackName()` - 获取合约后备名称

## 只使用 ui-core

如果你的项目不需要 Web3 功能,可以只安装和使用 `@sentio/ui-core`:

```bash
pnpm add @sentio/ui-core
```

```tsx
import { Button, BaseDialog, BarLoading } from '@sentio/ui-core'
import '@sentio/ui-core/dist/style.css'
```

## 架构

```
@sentio/ui-web3
    ↓ depends on & re-exports
@sentio/ui-core
    ↓ provides
基础组件 + 样式
```

用户可以:
1. 只装 `@sentio/ui-core` - 获取基础 UI 组件
2. 装 `@sentio/ui-web3` - 自动包含 ui-core + Web3 组件
