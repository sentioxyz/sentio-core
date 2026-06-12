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
// Components are added in subsequent changes (timeseries, charts, dashboard
// shells, then the decoupled dialogs/panels).
// ──────────────────────────────────────────────────────────────────────────
