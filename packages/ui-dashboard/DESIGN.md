---
name: Sentio Dashboard UI
version: "1.0"
description: Dashboard UI component library for the Sentio platform — a monitoring and observability dashboard system with multi-chart support, panel management, and dark/light mode.

colors:
  # ── Brand / Primary (indigo-blue) ────────────────────────────
  primary-50:  "#f2f5ff"
  primary-100: "#e8eeff"
  primary-200: "#d7e0ff"
  primary-300: "#b8c8ff"
  primary-400: "#8ea5ff"
  primary-500: "#5d7dff"
  primary-600: "#3b5fff"
  primary-700: "#2e4eeb"
  primary-800: "#223fd4"
  primary-900: "#1b2fa8"
  primary-950: "#111b66"

  # ── Semantic text ────────────────────────────────────────────
  text-foreground:         "#171321"
  text-foreground-secondary: "#625d75"
  text-foreground-tertiary:  "#908ca3"
  text-foreground-disabled:  "#b6b1c7"
  text-background:         "#ffffff"

  # ── Gray scale (purple-gray) ─────────────────────────────────
  gray-50:  "#fcfbfe"
  gray-100: "#f7f5fb"
  gray-200: "#f2eff8"
  gray-300: "#ece8f5"
  gray-400: "#e3dcf2"
  gray-500: "#ddd6eb"
  gray-600: "#d1ccdb"
  gray-700: "#b6b1c7"
  gray-800: "#908ca3"
  gray-900: "#625d75"
  gray-950: "#171321"

  # ── Background surfaces ──────────────────────────────────────
  bg-canvas:   "#fcfbfe"
  bg-surface:  "#f7f5fb"
  bg-elevated: "#f2eff8"
  bg-hover:    "#ece8f5"
  bg-active:   "#e3dcf2"

  # ── Borders ──────────────────────────────────────────────────
  border-main:   "#e5dfef"
  border-light:  "#ece8f5"
  border-dark:   "#ddd6eb"

  # ── Accent / semantic ────────────────────────────────────────
  red-600:    "#d84c36"
  orange-600: "#e8821a"
  yellow-600: "#d9a928"
  cyan-600:   "#159f6b"   # teal-green / success; name kept for backward compat — use cyan-600 for success states, not cyan hue
  purple-600: "#8b5cf6"

  # ── Brand extended ───────────────────────────────────────────
  daybreak-blue-600: "#1799fd"
  lake-blue-600:     "#0891b2"
  deep-purple-600:   "#7c3aed"
  magenta-600:       "#f36ad9"

  # ── Sentio gray (neutral, default) ──────────────────────────
  sentio-gray-50:  "#fafafa"
  sentio-gray-100: "#f5f5f5"
  sentio-gray-200: "#eeeeee"
  sentio-gray-300: "#e0e0e0"
  sentio-gray-400: "#bdbdbd"
  sentio-gray-500: "#9e9e9e"
  sentio-gray-600: "#757575"
  sentio-gray-700: "#616161"
  sentio-gray-800: "#424242"
  sentio-gray-900: "#212121"
  sentio-gray-950: "#090909"

  # ── Chart palette: classic (light) ──────────────────────────
  # 9-color categorical palette. Index order: blue, cyan, pink, yellow, green,
  # lightblue, purple, red, orange. Dark variant shifts saturation while
  # preserving hue mapping.
  chart-0: "#5470f0"   # blue
  chart-1: "#47c9d9"   # cyan
  chart-2: "#de5f94"   # pink
  chart-3: "#e4bc4f"   # yellow
  chart-4: "#4cb275"   # green
  chart-5: "#77aeef"   # lightblue
  chart-6: "#9368dd"   # purple
  chart-7: "#e46d6d"   # red
  chart-8: "#f1904e"   # orange

  # ── Chart palette: purple (light) ───────────────────────────
  chart-purple-0: "#5b0fa6"
  chart-purple-1: "#6d11c9"
  chart-purple-2: "#8617e8"
  chart-purple-3: "#9b35e9"
  chart-purple-4: "#a855f7"
  chart-purple-5: "#b67af2"
  chart-purple-6: "#7a6bff"
  chart-purple-7: "#5b7cff"
  chart-purple-8: "#3e82f6"

  # ── Dark mode overrides ─────────────────────────────────────
  dark:
    primary-600: "#5b7cff"
    text-foreground:         "#ffffff"
    text-foreground-secondary: "#b7b4c7"
    text-foreground-tertiary:  "#7d7893"
    text-foreground-disabled:  "#5c5870"
    text-background:         "#0b0714"
    bg-canvas:   "#0b0714"
    bg-surface:  "#11091f"
    bg-elevated: "#171028"
    bg-hover:    "#1e1633"
    bg-active:   "#241b3d"
    border-main:   "#2b2440"
    border-light:  "#362b4b"
    border-dark:   "#41394c"
    gray-50:   "#0b0714"
    gray-100:  "#11091f"
    gray-200:  "#171028"
    gray-300:  "#1e1633"
    gray-400:  "#241b3d"
    gray-500:  "#2b2440"
    gray-600:  "#362b4b"
    gray-700:  "#41394c"
    gray-800:  "#5c5870"
    gray-900:  "#7d7893"
    gray-950:  "#b7b4c7"
    chart-dark-0: "#6c8aff"
    chart-dark-1: "#74dfe6"
    chart-dark-2: "#ff75b0"
    chart-dark-3: "#f1cf66"
    chart-dark-4: "#67c88f"
    chart-dark-5: "#95c6ff"
    chart-dark-6: "#b189ff"
    chart-dark-7: "#f28787"
    chart-dark-8: "#ffad67"
    chart-dark-purple-0: "#3f0a78"
    chart-dark-purple-1: "#5310a0"
    chart-dark-purple-2: "#6816c7"
    chart-dark-purple-3: "#7c2ee6"
    chart-dark-purple-4: "#9451f4"
    chart-dark-purple-5: "#a874f8"
    chart-dark-purple-6: "#6d63f6"
    chart-dark-purple-7: "#5b7cff"
    chart-dark-purple-8: "#4794ff"

typography:
  body:
    fontFamily: "ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif"
    fontSize: "0.875rem"
    lineHeight: "1.25rem"
    fontWeight: "400"
  mono:
    fontFamily: "Menlo, ui-monospace, SFMono-Regular, 'SF Mono', Consolas, 'Liberation Mono', monospace"
  code:
    fontFamily: "'Fira Code', 'Fira Mono', Menlo, Consolas, 'DejaVu Sans Mono', ui-monospace, monospace"
  ichart:
    fontSize: "0.625rem"   # 10px
    lineHeight: "1rem"     # 16px
  icontent:
    fontSize: "0.8125rem"  # 13px
    lineHeight: "1.125rem" # 18px
    fontWeight: "400"
  ilabel:
    fontSize: "0.8125rem"  # 13px
    lineHeight: "1.125rem" # 18px
    fontWeight: "500"
  ititle:
    fontSize: "1.125rem"   # 18px
    lineHeight: "1.75rem"  # 28px
    fontWeight: "600"

spacing:
  xs: "0.25rem"    # 4px
  sm: "0.5rem"     # 8px
  md: "1rem"       # 16px
  lg: "1.5rem"     # 24px
  xl: "2rem"       # 32px
  2xl: "2.5rem"    # 40px

rounded:
  sm: "0.25rem"    # 4px
  md: "0.5rem"     # 8px
  lg: "0.75rem"    # 12px
  full: "9999px"

shadows:
  sm: "0 1px 2px 0 rgb(0 0 0 / 0.05)"
  md: "0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)"
  lg: "0 10px 15px -3px rgb(0 0 0 / 0.1), 0 4px 6px -4px rgb(0 0 0 / 0.1)"

zIndex:
  nav: 2
  dropdown: 50
  tooltip: 90
  modal: 100

animation:
  fade-in:
    keyframes:
      "0%": { opacity: "0", transform: "translateY(20px)" }
      "100%": { opacity: "1", transform: "translateY(0)" }
    duration: "0.6s"
    easing: "ease-out"
  float:
    keyframes:
      "0%, 100%": { transform: "translateY(10px)" }
      "50%": { transform: "translateY(-10px)" }
    duration: "3s"
    easing: "ease-in-out"
    iteration: "infinite"
  bounce-x:
    keyframes:
      "0%, 100%": { transform: "translateX(0)" }
      "50%": { transform: "translateX(4px)" }
    duration: "0.6s"
    easing: "ease-in-out"
    iteration: "infinite"

components:
  button-primary:
    textColor: "#ffffff"
    backgroundColor: "#3b5fff"
    rounded: "0.375rem"
    padding: "0.5rem 1rem"
    size: "auto"
    # Active: #223fd4, Hover: #2e4eeb, Disabled opacity: 0.5
    # Danger variant: bg #d84c36, hover #bd3a26, active #9a2d1c

  button-secondary:
    textColor: "#171321"
    backgroundColor: "transparent"
    rounded: "0.375rem"
    padding: "0.5rem 1rem"
    # Border: 1px solid #e5dfef
    # Hover: bg #f2f5ff, text #3b5fff, border #3b5fff
    # Disabled: text #b6b1c7
    # Danger variant: border+text #d84c36

  button-dashed:
    textColor: "#171321"
    backgroundColor: "transparent"
    rounded: "0.375rem"
    # Border: 1px dashed #ddd6eb
    # Hover: text+border #3b5fff

  button-text:
    textColor: "#171321"
    backgroundColor: "transparent"
    rounded: "0.375rem"
    # Hover: bg #f2f5ff, text #3b5fff

  button-link:
    textColor: "#3b5fff"
    backgroundColor: "transparent"
    # Hover: #5d7dff, Active: #2e4eeb

  button-tertiary:
    textColor: "#3b5fff"
    backgroundColor: "#e8eeff"
    rounded: "0.375rem"
    # Hover: bg #d7e0ff
    # Dark: text #b7b4c7, bg #11091f, hover bg #171028
    # Note: dark variant drops the primary tint — bg becomes neutral dark surface, not a tinted indigo.
    # This is intentional: the tint reads as selected/active in dark mode; tertiary should be subtle.

  input:
    textColor: "#171321"
    backgroundColor: "transparent"
    rounded: "0.25rem"
    # Border: 1px solid #ece8f5
    # Placeholder: #625d75
    # Padding: 0.375rem 0.75rem
    # Focus: border #5d7dff, box-shadow 0 0 0 1px #5d7dff
    # Disabled: bg #f2eff8

  checkbox:
    size: "1rem"
    # Border-radius: 0
    # Border: 1px solid #ece8f5
    # Checked: bg #3b5fff
    # Focus: box-shadow 0 0 0 2px #fff, 0 0 0 4px #5d7dff

  radio:
    size: "1rem"
    rounded: "9999px"
    # Border: 1px solid #ece8f5
    # Checked: bg #3b5fff

  slideover:
    width: "20rem"
    backgroundColor: "#ffffff"
    # Dark: bg #11091f

  dialog:
    backgroundColor: "#ffffff"
    rounded: "0.5rem"
    # Dark: bg #171028

  tooltip:
    backgroundColor: "transparent"
    # CSS class: .sentio-tooltip-item (padding: 0 10px)
    # Highlighted: bg #ece8f5 (light), #241b3d (dark)
    # Series name font-weight: bold

  group-header-highlights:
    - { key: "green", name: "Sentio Green", classicIndex: 4 }
    - { key: "blue", name: "Sentio Blue", classicIndex: 0 }
    - { key: "cyan", name: "Sentio Cyan", classicIndex: 1 }
    - { key: "lightblue", name: "Sentio Light Blue", classicIndex: 5 }
    - { key: "purple", name: "Sentio Purple", classicIndex: 6 }
    - { key: "pink", name: "Sentio Pink", classicIndex: 2 }
    - { key: "red", name: "Sentio Red", classicIndex: 7 }
    - { key: "orange", name: "Sentio Orange", classicIndex: 8 }
    - { key: "yellow", name: "Sentio Yellow", classicIndex: 3 }

---

## Sentio Dashboard UI — Design System

This library provides dashboard UI components for the **Sentio observability platform**. It inherits all theme tokens from `@sentio/ui-core` via a shared Tailwind config and adds dashboard-specific components on top.

### Design Philosophy

**Data-in, actions-out.** Components never make network requests or data hooks — all data flows in through props, and side effects are surfaced as callbacks (`onSave`, `onSearch`, `onNavigate`). This keeps the library decoupled from any backend.

**Structural contracts.** Components type their props against minimal structural interfaces (`PanelLike`, `ChartLike`, `DashboardLike`) rather than concrete data types. Consumers pass their own richer data objects, which are structurally assignable — this package never needs to depend on upstream data-model definitions.

**Theme inheritance.** The Tailwind configuration is literally a re-export of `@sentio/ui-core/tailwind.config.js`. All CSS custom properties (`--primary-*`, `--gray-*`, `--text-*`, `--bg-*`, `--border-*`) are defined in `@sentio/ui-core`'s `theme-variables.css` and loaded at runtime by the consumer. This package only adds component-specific utilities.

### Dark Mode

Dark mode is toggled by adding/removing the `dark` class on `<body>`:

```css
body.dark {
  --text-foreground: 255, 255, 255;
  --text-background: 11, 7, 20;
  --bg-canvas: 11, 7, 20;
  /* … all other tokens switch to dark values */
}
```

The `useDarkMode()` hook (in `src/utils/use-dark-mode.ts`) is **read-only**: it observes the body class via a `MutationObserver` and returns `isDarkMode: boolean`. Use it anywhere you need to react to the current mode. The `useSetDarkMode()` hook is **write-only**: it provides `toggle`, `onChange(value)` controls with `'light' | 'dark' | 'system'` options, persisting the choice in `localStorage`. Only use `useSetDarkMode` in the component that owns the mode toggle — avoid calling it in leaf components.

### Color Philosophy

The palette is rooted in **purple-gray neutrals** with a **vibrant indigo-blue primary** for interaction. Colors use CSS custom properties with RGB triplets (e.g. `--primary-600: 59, 95, 255; rgba(var(--primary-600))`) so opacity can be applied at usage time via `rgba()`.

- **Primary** (`--primary-*`): Indigo-blue — the single driving interaction color. Used for buttons, focus rings, links, active states, checkboxes, toggles.
- **Gray** (`--gray-*`): Purple-gray scale — backgrounds, borders, disabled text.
- **Sentio Gray** (`--sentio-gray-*`): Neutral warm gray — an alternative when a true neutral is needed.
- **Semantic text** (`--text-foreground`, `--text-foreground-secondary`, `--text-foreground-disabled`): Three-tier hierarchy for content.
- **Background surfaces** (`--bg-canvas`, `--bg-surface`, `--bg-elevated`, `--bg-hover`, `--bg-active`): Five-layer elevation system for depth.
- **Borders** (`--border-main`, `--border-light`, `--border-dark`): Three-tier border system.
- **Accent colors** (red, orange, yellow, cyan/teal-green, purple): Used for status badges, semantic feedback, danger variants.
- **Brand extended** (daybreak-blue, lake-blue, deep-purple, magenta): Additional brand palette members.
- **Chart palette** (classic: 9 colors): The categorical palette for ECharts visualizations, with separate light and dark variants that shift saturation while preserving hue mapping. Light palette tokens are top-level (`chart-0` … `chart-8`, `chart-purple-*`); dark overrides are nested under `dark:` with a `chart-dark-` prefix (`chart-dark-0` … `chart-dark-8`, `chart-dark-purple-*`). The naming asymmetry is intentional — dark tokens carry `dark-` in their name to avoid collision when both palettes are referenced in the same scope.

### Typography

| Token | Size | Weight | Usage |
|-------|------|--------|-------|
| `ichart` | 10px/16px | 400 | Chart labels and axis text |
| `icontent` | 13px/18px | 400 | Body content text |
| `ilabel` | 13px/18px | 500 | Section headers, labels |
| `ititle` | 18px/28px | 600 | First-level headers |

The base sans stack is system-native: `ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif`. Monospace is `Menlo` with standard fallbacks. Code blocks use `'Fira Code', 'Fira Mono'` before the Menlo fallback.

### Component Architecture

#### Package dependency chain

```
@sentio/ui-core  (theme tokens, base components: Button, Dialog, Select, Input, PopupMenu, SlideOver, Tabs, icons)
       ↑
@sentio/ui-dashboard  (dashboard+chart components, reuses ui-core theme)
```

#### Dashboard components (`src/dashboard/`)

| Component | Purpose |
|-----------|---------|
| `DashboardTitle` | Dropdown selector for switching between dashboards |
| `GroupPanel` | Individual dashboard group/panel container with header highlight |
| `AddPanel` / `AddPanelSlideover` | Panel creation UI (Lines, Bars, Areas, Pie, Bar Gauge, Scatter, Table, Query Value, Note, SQL) |
| `EditDashboardDialog` | Dashboard metadata editing |
| `EditGroupDialog` | Group/panel header configuration |
| `CreateDashboardDialog` | New dashboard creation |
| `DashboardRefresh` | Refresh trigger with loading state |
| `DashboardButtons` | Dashboard-level action toolbar |
| `ShareDashboardDialog` / `ExportDashboardDialog` / `ImportDashboardDialog` / `ImportPanelDialog` | Data sharing and import/export workflows |
| `CurlDialog` | cURL code generation for API access |
| `PanelOwner` | Panel ownership display |
| `SeriesControls` | Series-level configuration (aliases, colors) |
| `QueryValueControls` | Query value visualization options |
| `TimeRangeOverride` | Per-panel time range override |
| `ExtraSettingMenu` | Panel's extra settings menu |
| `ExportChartMenu` | Chart export controls |
| `ErrorChart` | Error state fallback for chart panels |

#### Chart components (`src/charts/`)

| Component | Purpose |
|-----------|---------|
| `ReactEChartsBase` | Core ECharts wrapper — handles init/resize/theme/lifecycle. Registers `sentio` / `sentio-dark` themes. Supports canvas and SVG renderers. |
| `TimeSeriesChart` | Line, bar, area, scatter time-series visualization |
| `PieChart` | Pie/donut chart visualization |
| `BarGaugeChart` | Horizontal bar gauge visualization |
| `QueryValueChart` | Single value / stat display |
| `ChartTooltip` / `ScatterChartTooltip` | Crosshair tooltip component |
| `ChartLegend` | Custom legend rendering |
| `ChartTypeButtonGroup` | Chart type selector (line/bar/area/pie/etc.) |
| `RefreshContext` | Auto-refresh context provider |

#### ECharts Theme (`src/charts/theme/`)

Two registered themes: `sentio` (light) and `sentio-dark` (dark). They control colors for lines, bars, pies, scatters, candlesticks, axes, grids, tooltips, legends, toolboxes, data zooms, and visual maps. The classic palette (9 colors) is the default series color cycle; a purple variant is also available.

### Spacing & Layout

The dashboard uses `react-grid-layout` for panel positioning. Panels are organized into **Groups** (collapsible sections with optional header highlight colors from the chart palette). The layout system supports:

- Drag-and-drop panel rearrangement
- Panel resizing
- Group-level time range overrides
- Row-based grid with auto-height

### Group Header Highlights

Groups can be emphasized with a colored header bar. Colors map to the chart classic palette indices and shift with dark/light mode automatically:

```
EMPHASIS  →  classic[0]  blue
HIGHLIGHT →  classic[2]  pink
WARNING   →  classic[7]  red
```

When a highlight color is applied, the foreground text color is computed via W3C relative luminance to ensure WCAG AA contrast (returns `#1f2937` for light backgrounds, `#ffffff` for dark).

### States

- **Loading**: `BarLoading` spinner from `@sentio/ui-core`, shown in chart panels while data loads
- **Empty**: Handled by individual chart components — no shared empty-state component by design; each chart type renders its own empty treatment
- **Error**: `ErrorChart` component renders in place of a failed panel
- **Disabled**: Buttons use `.btn-disabled` class with reduced opacity (`0.5`) and `cursor-not-allowed`
- **Focus**: Custom ring utilities (`ring-primary`) — native `:focus-visible` outline is globally suppressed in favor of component-level opt-in focus halos

### Iconography

Uses `react-icons` (`LuChevronDown`, `VscAdd`) for generic icons, plus custom SVG chart-type icons (`LineIcon`, `AreaIcon`, `BarIcon`, `PieIcon`, `ScatterIcon`, `TableIcon`, `BarGuageIcon`, `QueryValueIcon`). Dashboard panel-type picker icons come from `@sentio/ui-core` (`LinesIcon`, `BarsIcon`, `AreasIcon`, `GaugeIcon`, `GroupsIcon`, `NoteIcon`, `PieIcon`, `QueryValueIcon`, `TableIcon`, `SQlIcon`, `EventLogsIcon`, `ScatterIcon`, `ImportIcon`).

### Animations

Primary animations are fade-in (entrance, `0.6s ease-out`), float (gentle vertical hover, `3s infinite`), and bounce-x (horizontal pulse, `0.6s infinite`). Chart interactions use ECharts' built-in animation system.
