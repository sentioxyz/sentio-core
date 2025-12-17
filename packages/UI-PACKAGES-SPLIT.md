# Sentio UI 组件库 - 包拆分说明

## 概述

原来的 `@sentio/ui-components` 已被拆分为两个包:

1. **@sentio/ui-core** - 基础 UI 组件,不依赖 Web3
2. **@sentio/ui-web3** - Web3 专用组件,依赖链语义

## 拆分原则

### ✅ 实现的目标

1. ✅ **ui-core 完全不依赖 web3**
   - 移除了所有 `web3`、`web3-utils` 依赖
   - 只包含纯 UI 组件和通用工具

2. ✅ **ui-web3 只在需要链语义的组件里出现**
   - 所有 transaction 相关组件
   - 地址标签、价格查询等 Web3 特定功能

3. ✅ **用户可以只装 ui-core**
   - 完全独立的包
   - 提供完整的基础 UI 能力

4. ✅ **ui-web3 自动复用 ui-core**
   - 通过 `export * from '@sentio/ui-core'` 重新导出
   - 自动继承样式和基础组件

## 包结构

```
packages/
├── ui-core/                 # 基础 UI 组件
│   ├── src/
│   │   ├── common/         # 基础组件(Button, Dialog, Loading 等)
│   │   ├── utils/          # 通用工具(number-format, use-mobile, extension-context)
│   │   ├── styles.css      # Tailwind 样式
│   │   ├── theme-variables.css
│   │   └── index.ts
│   ├── package.json        # 无 web3 依赖
│   └── README.md
│
└── ui-web3/                # Web3 组件
    ├── src/
    │   ├── transaction/    # 交易相关组件(BalanceChanges, HexNumber 等)
    │   ├── utils/          # Web3 工具(use-tag, use-price)
    │   └── index.ts        # 重新导出 ui-core + web3 组件
    ├── package.json        # 依赖 @sentio/ui-core
    └── README.md
```

## 组件分类

### @sentio/ui-core

**基础组件:**
- `Button` - 按钮
- `BaseDialog` - 对话框
- `PopoverTooltip` - 提示框
- `BarLoading` / `SpinLoading` - 加载指示器
- `CopyButton` - 复制按钮

**工具:**
- `useMobile()` - 移动设备检测
- `getNumberWithDecimal()` - 数字格式化
- `parseHex()` - 十六进制解析
- Contexts: `SvgFolderContext`, `DarkModeContext`, `OpenContractContext`

### @sentio/ui-web3

**交易组件:**
- `BalanceChanges` - 余额变化
- `HexNumber` - 地址/哈希展示
- `TransactionStatus` - 交易状态
- `TransactionValue` - 交易金额
- `AddressFrom` / `AddressTo` - 地址展示
- `TransactionLabel` - 交易标签

**Web3 工具:**
- `useAddressTag()` - 地址标签
- `usePrice()` - 代币价格
- `useFallbackName()` - 合约名称

**+ 所有 ui-core 的组件和工具**

## 使用方式

### 方式 1: 只使用基础组件

适用于不需要 Web3 功能的项目:

```bash
pnpm add @sentio/ui-core
```

```tsx
import { Button, BaseDialog, BarLoading } from '@sentio/ui-core'
import '@sentio/ui-core/dist/style.css'
```

### 方式 2: 使用 Web3 组件

适用于需要展示区块链数据的项目:

```bash
pnpm add @sentio/ui-web3  # 自动包含 @sentio/ui-core
```

```tsx
// 可以从 ui-web3 导入所有组件(包括 ui-core 的)
import { 
  Button,           // 来自 ui-core
  BalanceChanges,   // 来自 ui-web3
  HexNumber         // 来自 ui-web3
} from '@sentio/ui-web3'

import '@sentio/ui-core/dist/style.css'
```

## 依赖关系

```mermaid
graph TD
    A[用户项目] --> B[@sentio/ui-web3]
    B --> C[@sentio/ui-core]
    B --> D[web3, @sentio/chain, etc.]
    C --> E[react, headlessui, etc.]
    
    F[纯 UI 项目] --> C
```

## 迁移指南

### 从旧的 @sentio/ui-components 迁移

**如果只使用基础组件:**
```diff
- import { Button, Dialog } from '@sentio/ui-components'
+ import { Button, BaseDialog } from '@sentio/ui-core'
- import '@sentio/ui-components/dist/style.css'
+ import '@sentio/ui-core/dist/style.css'
```

**如果使用 Web3 组件:**
```diff
- import { Button, BalanceChanges } from '@sentio/ui-components'
+ import { Button, BalanceChanges } from '@sentio/ui-web3'
- import '@sentio/ui-components/dist/style.css'
+ import '@sentio/ui-core/dist/style.css'
```

## 开发

### 构建 ui-core

```bash
cd packages/ui-core
pnpm install
pnpm build
```

### 构建 ui-web3

```bash
cd packages/ui-web3
pnpm install  # 会自动 link ui-core
pnpm build
```

### 开发模式

```bash
# Terminal 1: 启动 ui-core 开发模式
cd packages/ui-core
pnpm dev

# Terminal 2: 启动 ui-web3 开发模式
cd packages/ui-web3
pnpm dev
```

## 包大小对比

- **@sentio/ui-core**: ~150KB (不含 web3 依赖)
- **@sentio/ui-web3**: ~500KB (包含 web3 相关依赖)

如果只使用基础组件,可以节省约 70% 的包体积!
