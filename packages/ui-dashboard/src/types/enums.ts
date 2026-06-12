/*
 * String-literal unions for dashboard chart/panel enums.
 *
 * Using unions instead of TS enums keeps these as pure types (zero runtime
 * code) and makes them structurally assignable from a consumer's own
 * string-valued enums — e.g. a value of `"LINE"` assigns to `ChartTypeLike`.
 *
 * Only the values the components actually branch on are included; widening is
 * a deliberate, reviewed change.
 */

/** Dashboard chart type. */
export type ChartTypeLike =
  | 'LINE'
  | 'AREA'
  | 'BAR'
  | 'BAR_GAUGE'
  | 'TABLE'
  | 'QUERY_VALUE'
  | 'PIE'
  | 'NOTE'
  | 'SCATTER'
  | 'GROUP'

/** Data source backing a chart. */
export type DataSourceTypeLike =
  | 'METRICS'
  | 'NOTES'
  | 'ANALYTICS'
  | 'INSIGHTS'
  | 'EVENTS'
  | 'RETENTION'
  | 'SQL'
  | 'GROUP'

/** Dashboard visibility scope. */
export type DashboardVisibilityLike = 'INTERNAL' | 'PRIVATE' | 'PUBLIC'

/** Visual style for a group panel header. */
export type GroupStyleLike = 'DEFAULT' | 'EMPHASIS'

/** Note panel font size. */
export type NoteFontSizeLike = 'MD' | 'SM' | 'LG' | 'XL' | 'XXL'

/** Note panel horizontal alignment. */
export type NoteAlignmentLike = 'LEFT' | 'CENTER' | 'RIGHT'

/** Note panel vertical alignment. */
export type NoteVerticalAlignmentLike = 'TOP' | 'MIDDLE' | 'BOTTOM'
