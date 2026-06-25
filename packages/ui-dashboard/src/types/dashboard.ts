import type { ResponsiveLayoutsLike } from './layout'
import type { ChartLike } from './chart'
import type { DashboardVisibilityLike, GroupStyleLike } from './enums'

/*
 * Dashboard / Panel / Group / Extra shapes — minimal structural interfaces for
 * the dashboard data model. All fields are optional so a consumer can pass its
 * own (richer) data objects directly; they assign structurally:
 *
 *     <DashboardPanel panel={panel as PanelLike} />
 */

/** A per-user template variable. */
export interface TemplateVariableLike {
  field?: string
  defaultValue?: string
  sourceName?: string
  options?: string[]
}

/** A saved set of template-variable values. */
export interface TemplateViewLike {
  values?: { [key: string]: string }
}

/** Extra dashboard configuration (template variables / saved views). */
export interface DashboardExtraLike {
  templateVariables?: { [key: string]: TemplateVariableLike }
  templateViews?: TemplateViewLike[]
}

/**
 * Config for a GROUP-typed panel — a collapsible container holding other
 * panels that reference it via `Panel.groupId`.
 */
export interface GroupLike {
  title?: string
  collapsed?: boolean
  childLayouts?: ResponsiveLayoutsLike
  style?: GroupStyleLike
  /** Palette key (e.g. "green", "purple"); empty keeps the theme default. */
  highlightColor?: string
}

/** A dashboard panel. `creator`/`updater` user info kept opaque for now. */
export interface PanelLike {
  id?: string
  name?: string
  dashboardId?: string
  chart?: ChartLike
  /** Non-empty => renders inside the group panel with this id. */
  groupId?: string
  creator?: unknown
  updater?: unknown
}

/** A dashboard. `sharing` kept opaque until the share dialog is migrated. */
export interface DashboardLike {
  id?: string
  name?: string
  projectId?: string
  description?: string
  panels?: { [id: string]: PanelLike }
  layouts?: ResponsiveLayoutsLike
  extra?: DashboardExtraLike
  default?: boolean
  isPinned?: boolean
  visibility?: DashboardVisibilityLike
  ownerId?: string
  tags?: string[]
  url?: string
  projectOwner?: string
  projectSlug?: string
  createPanels?: string[]
  editPanels?: string[]
  sharing?: unknown
}

// Compute freshness stats for a dashboard's data (proto common.ComputeStats).
// computedAt is the RFC3339 timestamp string from the JSON wire format.
export interface ComputeStatsLike {
  computedAt?: string
  isRefreshing?: boolean
}
