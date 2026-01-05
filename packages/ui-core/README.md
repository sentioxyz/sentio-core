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
import { 
  Button, 
  BaseDialog, 
  BarLoading, 
  CopyButton,
  Input,
  Select,
  Switch,
  PopoverTooltip,
  ResizeTable,
  FlatTree,
  useMobile
} from '@sentio/ui-core'
import '@sentio/ui-core/dist/style.css'

function App() {
  const isMobile = useMobile()
  
  return (
    <>
      <Button>Click me</Button>
      <BarLoading />
      <CopyButton text="Copy this" />
      <Input placeholder="Enter text" />
      {!isMobile && <p>Desktop view</p>}
    </>
  )
}
```

## Included components

### Common Components
- `BarLoading` - Bar loading indicator
- `SpinLoading` - Spinner loading indicator
- `CopyButton`, `CopyIcon`, `CopySuccessIcon` - Copy button and icons
- `Button` - Button component with loading state support
- `BaseDialog`, `BaseZIndexContext` - Dialog component with z-index context
- `PopoverTooltip` - Tooltip component
- `DisclosurePanel` - Disclosure panel component
- `Collapse` - Collapse/expand component
- `Input` - Input component
- `RadioSelect` - Radio select component
- `Switch` - Switch toggle component
- `Select` - Select dropdown component
- `FlatTree` - Tree component with flat data structure
- `LinkifyText` - Text component that converts URLs to links
- `Empty` - Empty state component
- `StatusBadge`, `StatusRole` - Status badge components
- `HeaderToolsToggleButton`, `HeaderToolsContent` - Header tools dropdown

### Table Components
- `ResizeTable` - Resizable table component
- `MoveLeftIcon`, `MoveRightIcon`, `RenameIcon`, `DeleteIcon` - Table action icons

### Menu Components
- `PopupMenuButton` - Popup menu button
- `MenuItem`, `SubMenuButton`, `MenuContext`, `COLOR_MAP` - Menu system components

### Tree Components
- `ROOT_KEY`, `SUFFIX_NODE_KEY` - Tree node key constants
- `PlusSquareO`, `MinusSquareO`, `CloseSquareO`, `EyeO` - Tree icons

### Utilities & Hooks
- `useMobile()` - Detect mobile device
- `useBoolean()` - Boolean state hook
- Number formatting utilities (e.g., `getNumberWithDecimal()`)
- `classNames()` - Classname utility
- Contexts: `NavSizeContext`, `BaseZIndexContext`, `MenuContext`
- Extension context utilities

## Theming

Components are themed using CSS variables. You can customize the theme by overriding these variables:

```css
:root {
  --primary-600: 7, 86, 213;
  --gray-600: 75, 85, 99;
  /* ... */
}
```
