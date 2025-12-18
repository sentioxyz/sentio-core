# @sentio/ui-core

A basic UI component library with zero Web3 dependencies.

## Features

- 🎨 Full Tailwind CSS theme system
- 🧩 Core UI components (Button, Dialog, Tooltip, Loading, etc.)
- 📦 No Web3 dependency
- 🎯 Lightweight

## Installation

```bash
pnpm add @sentio/ui-core
```

## Usage

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

## Included components

### Core components
- `Button` - Button component
- `BaseDialog` - Dialog component
- `PopoverTooltip` - Tooltip component
- `BarLoading` - Bar loading indicator
- `SpinLoading` - Spinner loading indicator
- `CopyButton` - Copy button

### Utilities / Hooks
- `useMobile()` - Detect mobile device
- `getNumberWithDecimal()` - Number formatting
- `parseHex()` - Hex parsing
- Contexts: `SvgFolderContext`, `DarkModeContext`, `OpenContractContext`

## Theming

Components are themed using CSS variables. You can customize the theme by overriding these variables:

```css
:root {
  --primary-600: 7, 86, 213;
  --gray-600: 75, 85, 99;
  /* ... */
}
```
