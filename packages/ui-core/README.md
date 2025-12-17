# @sentio/ui-core

基础 UI 组件库,完全不依赖 Web3。

## 特性

- 🎨 完整的 Tailwind CSS 主题系统
- 🧩 基础 UI 组件(Button, Dialog, Tooltip, Loading 等)
- 📦 零 Web3 依赖
- 🎯 轻量级

## 安装

```bash
pnpm add @sentio/ui-core
```

## 使用

```tsx
import { Button, BaseDialog, BarLoading, CopyButton } from '@sentio/ui-core'
import '@sentio/ui-core/dist/style.css'

function App() {
  return (
    <>
      <Button>Click me</Button>
      <BarLoading />
      <CopyButton text="Copy this" />
    </>
  )
}
```

## 包含的组件

### 基础组件
- `Button` - 按钮组件
- `BaseDialog` - 对话框组件
- `PopoverTooltip` - 提示框组件
- `BarLoading` - 条形加载指示器
- `SpinLoading` - 旋转加载指示器
- `CopyButton` - 复制按钮

### 工具函数
- `useMobile()` - 检测移动设备
- `getNumberWithDecimal()` - 数字格式化
- `parseHex()` - 十六进制解析
- Context: `SvgFolderContext`, `DarkModeContext`, `OpenContractContext`

## 主题

组件使用 CSS 变量进行主题化,你可以通过覆盖这些变量来自定义主题:

```css
:root {
  --primary-600: 7, 86, 213;
  --gray-600: 75, 85, 99;
  /* ... */
}
```
