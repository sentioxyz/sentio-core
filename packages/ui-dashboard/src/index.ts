import './styles.css'

// ──────────────────────────────────────────────────────────────────────────
// Type contracts
//
// Minimal, structurally-typed interfaces for the dashboard data model. Public
// components type their props against these so a consumer can pass its own
// (richer) data objects directly without this package depending on the
// consumer's data types. See src/types/*.ts for the per-shape rationale.
// ──────────────────────────────────────────────────────────────────────────
export type {
  // enums
  ChartTypeLike,
  DataSourceTypeLike,
  DashboardVisibilityLike,
  GroupStyleLike,
  NoteFontSizeLike,
  NoteAlignmentLike,
  NoteVerticalAlignmentLike,
  // layout
  LayoutItemLike,
  LayoutsLike,
  ResponsiveLayoutsLike,
  // chart
  YAxisConfigLike,
  ChartConfigLike,
  NoteLike,
  OverlayGraphLike,
  ChartLike,
  // dashboard
  TemplateVariableLike,
  TemplateViewLike,
  DashboardExtraLike,
  GroupLike,
  PanelLike,
  DashboardLike
} from './types'

// ──────────────────────────────────────────────────────────────────────────
// Components
//
// timeseries: the metrics-query form (aggregate / labels / functions inputs)
// plus its domain helpers (function definitions, system labels, label-search
// context). charts / dashboard shells / decoupled dialogs are added later.
// ──────────────────────────────────────────────────────────────────────────
export * from './timeseries'

// charts: ECharts base/legend/refresh + theme + chart-type icons (phase 3a).
// Render charts and option panels follow in later phases.
export * from './charts'

// dashboard: presentational dialogs/shells (router & data wired via callbacks,
// proto types via *Like).
export * from './dashboard'
