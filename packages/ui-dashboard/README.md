# @sentio/ui-dashboard

Dashboard UI components for the Sentio platform. Sits one layer above
[`@sentio/ui-core`](../ui-core): it may depend on ui-core and on **dashboard
data-type contracts**, but it never makes network requests.

## Design rules

- **No network requests / no data hooks.** All data comes in through props;
  side effects are surfaced as callbacks (`onSave`, `onSearch`, `onNavigate`…).
- **No consumer data types.** Components type their props against the minimal
  structural interfaces in `src/types` (e.g. `PanelLike`, `ChartLike`,
  `DashboardLike`). A consumer passes its own (richer) data objects directly —
  they are structurally assignable, so this package never needs to depend on
  the consumer's data-type definitions.
- **Shared theme.** Reuses ui-core's Tailwind theme verbatim
  (`tailwind.config.js` re-exports ui-core's). Theme CSS variables are provided
  at runtime by ui-core's `style.css`.

## Usage

```tsx
import '@sentio/ui-core/dist/style.css'
import '@sentio/ui-dashboard/dist/style.css'

import type { PanelLike } from '@sentio/ui-dashboard'
```

## Type contracts

`src/types/*` are **minimal subsets** of the dashboard data model — only the
fields the components actually read. This keeps most upstream data-model
changes (new fields, new shapes) zero-sync for this package: components only
break when a field they actually depend on changes, which a consumer's own
type checker surfaces at its call sites.
