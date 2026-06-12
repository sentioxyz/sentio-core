/*
 * Responsive grid-layout shapes (react-grid-layout compatible). All fields are
 * optional so consumer-provided layout objects assign structurally.
 */

/** One panel's box in the grid. */
export interface LayoutItemLike {
  /** Panel id this layout entry belongs to. */
  i?: string
  x?: number
  y?: number
  w?: number
  h?: number
}

/** A set of layout items. */
export interface LayoutsLike {
  layouts?: LayoutItemLike[]
}

/** Layouts keyed by responsive breakpoint. */
export interface ResponsiveLayoutsLike {
  responsiveLayouts?: { [breakpoint: string]: LayoutsLike }
}
