import { Responsive } from 'react-grid-layout'
import type { ReactNode } from 'react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  LuChevronDown,
  LuChevronRight,
  LuPencil,
  LuTrash2
} from 'react-icons/lu'
import { mapValues, pick, sortBy, uniqBy } from 'lodash'
import { useResizeDetector } from 'react-resize-detector'
import { useAtomValue, useSetAtom } from 'jotai'
import {
  childDragOutAtom,
  classNames,
  findGroupAtPoint,
  groupDropZonesAtom,
  groupHoverTargetAtom,
  ROOT_ROW_HEIGHT
} from '@sentio/ui-core'
import { EditGroupDialog } from './EditGroupDialog'
import { resolveHeaderStyle } from './group-styles'
import { useDarkMode } from '../utils/use-dark-mode'
import type {
  DashboardLike,
  GroupStyleLike,
  LayoutItemLike,
  PanelLike,
  ResponsiveLayoutsLike
} from '../types'

interface Props {
  dashboard: DashboardLike
  panel: PanelLike
  childPanels: PanelLike[]
  allowEdit?: boolean
  // Number of columns the group occupies in the outer grid (drives inner cols).
  outerCols: number
  // Render a single child panel (NotePanel vs DashboardPanel branch lives in app).
  renderChild: (panel: PanelLike) => ReactNode
  onChildLayoutChanged?: (
    groupId: string,
    layouts: ResponsiveLayoutsLike
  ) => void
  onCollapseToggle?: (groupId: string, collapsed: boolean) => void
  // Edit dialog (title + style + highlight color) commits everything together.
  onConfigChange?: (
    groupId: string,
    patch: { title?: string; style?: GroupStyleLike; highlightColor?: string }
  ) => void
  onRemoveGroup?: (groupId: string) => void
  // Move a child panel from this group either out to the dashboard root
  // (targetGroupId === '') or into another group. Triggered by drag-out.
  onMoveChildOut?: (
    panelId: string,
    fromGroupId: string,
    targetGroupId: string
  ) => void
}

// Match the outer grid's row height so a collapsed group (h=1) is exactly
// filled by its header and the header's vertically-centered controls line up
// with one grid row.
const HEADER_HEIGHT_PX = ROOT_ROW_HEIGHT

const InnerGridProps: Partial<React.ComponentProps<typeof Responsive>> = {
  breakpoints: { md: 768, xxs: 0 },
  rowHeight: 60,
  margin: [12, 12],
  draggableHandle: '.draggableHandle',
  draggableCancel: '.nonDraggable',
  // Allow resizing from both bottom corners (default is bottom-right only).
  resizeHandles: ['se', 'sw'],
  useCSSTransforms: false
  // ponytail: dropped measureBeforeMount — it's a WidthProvider prop, no-op on
  // Responsive since we feed `width` manually.
  // compactType is driven dynamically on the JSX below to freeze the inner
  // grid layout while a child is being dragged.
}

function toServerLayouts(layouts: {
  [k: string]: LayoutItemLike[]
}): ResponsiveLayoutsLike {
  return {
    responsiveLayouts: mapValues(layouts, (layout) => ({
      layouts: layout.map((l) => pick(l, ['i', 'x', 'y', 'w', 'h']))
    }))
  }
}

export function GroupPanel({
  dashboard,
  panel,
  childPanels,
  allowEdit,
  outerCols,
  renderChild,
  onChildLayoutChanged,
  onCollapseToggle,
  onConfigChange,
  onRemoveGroup,
  onMoveChildOut
}: Props) {
  const group = panel.chart?.group
  const groupId = panel.id || ''

  // Collapse + title are tracked as LOCAL state so toggling re-renders only
  // this GroupPanel (instant) instead of waiting on the persist→clone→
  // full-dashboard-rerender round-trip. They re-sync from props whenever the
  // server reconciles (or another client changes them).
  const [collapsed, setCollapsed] = useState(!!group?.collapsed)
  useEffect(() => {
    setCollapsed(!!group?.collapsed)
  }, [group?.collapsed])

  const propTitle = group?.title || 'Group'
  const [title, setTitle] = useState(propTitle)
  useEffect(() => {
    setTitle(propTitle)
  }, [propTitle])

  // Style + highlight color are also tracked locally so the dialog's Save
  // applies instantly without waiting for the persist→re-render round trip.
  const propStyle = group?.style || 'DEFAULT'
  const propHighlight = group?.highlightColor || ''
  const [style, setStyle] = useState<GroupStyleLike>(propStyle)
  const [highlight, setHighlight] = useState<string>(propHighlight)
  useEffect(() => {
    setStyle(propStyle)
  }, [propStyle])
  useEffect(() => {
    setHighlight(propHighlight)
  }, [propHighlight])

  const [editDialogOpen, setEditDialogOpen] = useState(false)
  const ignoreLayoutChange = useRef(true)

  const isDark = useDarkMode()
  const headerStyle = useMemo(
    () => resolveHeaderStyle(style, highlight, isDark),
    [style, highlight, isDark]
  )
  const isStyled = style !== 'DEFAULT'
  // EMPHASIS centers the title in the tinted bar (matches the dialog's
  // Emphasis preview); DEFAULT keeps it left-aligned.
  const centerTitle = style === 'EMPHASIS'

  // Own the body ref directly and feed it to react-resize-detector via
  // targetRef — the hook's returned `ref` is a callback ref in v12 and
  // assigning .current on it does nothing.
  const bodyNodeRef = useRef<HTMLDivElement | null>(null)
  const { width: bodyWidth } = useResizeDetector<HTMLDivElement>({
    targetRef: bodyNodeRef,
    handleWidth: true,
    handleHeight: false
  })

  // Cache the last valid width so that when we unhide the body after collapse
  // the Responsive grid immediately gets the correct width (ResizeObserver
  // fires asynchronously on un-hide, so there's a 1-frame gap).
  const lastBodyWidthRef = useRef<number>(600)
  useEffect(() => {
    if (bodyWidth && bodyWidth > 0) lastBodyWidthRef.current = bodyWidth
  }, [bodyWidth])
  const effectiveBodyWidth =
    bodyWidth && bodyWidth > 0 ? bodyWidth : lastBodyWidthRef.current

  const dropZones = useAtomValue(groupDropZonesAtom)
  const setDropZones = useSetAtom(groupDropZonesAtom)
  const hoverTarget = useAtomValue(groupHoverTargetAtom)
  const setHoverTarget = useSetAtom(groupHoverTargetAtom)
  const setChildDragOut = useSetAtom(childDragOutAtom)
  const isHoverTarget = hoverTarget === groupId && !!groupId

  // Register this Group's body as a drop target so the root grid can hit-test
  // against it when a panel is dropped over it. Also re-registers when the
  // group expands (the body only exists while !collapsed).
  useEffect(() => {
    if (!groupId || collapsed) return
    setDropZones((prev) => ({
      ...prev,
      [groupId]: {
        id: groupId,
        getRect: () => bodyNodeRef.current?.getBoundingClientRect() ?? null
      }
    }))
    return () => {
      setDropZones((prev) => {
        if (!prev[groupId]) return prev
        const { [groupId]: _, ...rest } = prev
        return rest
      })
    }
  }, [collapsed, groupId, setDropZones])

  // Inner grid cols mirror the group's outer column span so childPanels can never
  // overflow the group horizontally.
  const innerCols = useMemo(
    () => ({ md: Math.max(1, outerCols), xxs: 1 }),
    [outerCols]
  )

  // Pre-sort childPanels by their stored layout to avoid layout shift on mount.
  const [sortedChildren, childLayouts] = useMemo(() => {
    const stored = group?.childLayouts?.responsiveLayouts || {}
    const layoutsByBp: { [k: string]: LayoutItemLike[] } = mapValues(
      stored,
      (l) =>
        uniqBy(
          sortBy(
            (l.layouts || []).map((item) => ({ ...item, minW: 1 })),
            ['y', 'x']
          ),
          'i'
        )
    )
    const mdLayout = layoutsByBp['md'] || []
    const byId: Record<string, PanelLike> = Object.fromEntries(
      childPanels.map((c) => [c.id || '', c])
    )
    const seen = new Set<string>()
    const sorted: PanelLike[] = []
    mdLayout.forEach((l) => {
      if (l.i && byId[l.i]) {
        sorted.push(byId[l.i])
        seen.add(l.i)
      }
    })
    // Append any unlayed-out childPanels at the bottom of the inner grid.
    let nextY = mdLayout.reduce(
      (m, l) => Math.max(m, (l.y || 0) + (l.h || 0)),
      0
    )
    childPanels.forEach((c) => {
      if (c.id && !seen.has(c.id)) {
        sorted.push(c)
        layoutsByBp['md'] = layoutsByBp['md'] || []
        layoutsByBp['md'].push({
          i: c.id,
          x: 0,
          y: nextY,
          w: Math.min(6, outerCols),
          h: 4
        })
        nextY += 4
      }
    })
    return [sorted, layoutsByBp]
  }, [childPanels, group?.childLayouts, outerCols])

  const onUserActionStart = useCallback(() => {
    ignoreLayoutChange.current = false
  }, [])

  // Defer mid-drag inner saves; commit (or discard in favor of a reparent) at
  // onDragStop. Same pattern as the root grid in Dashboard.tsx.
  const pendingDragLayoutRef = useRef<ResponsiveLayoutsLike | null>(null)
  const isDraggingRef = useRef(false)
  // True while the currently-dragged child has left this group's body (heading
  // to root or another group). Used to hide THIS group's inner placeholder so
  // there aren't two drop shadows at once (inner + the root/other-group one).
  const [childOutside, setChildOutside] = useState(false)
  const dropZonesRef = useRef(dropZones)
  useEffect(() => {
    dropZonesRef.current = dropZones
  }, [dropZones])

  const onInnerLayoutChange = useCallback(
    (_: any, layouts: { [k: string]: LayoutItemLike[] }) => {
      if (isDraggingRef.current) {
        pendingDragLayoutRef.current = toServerLayouts(layouts)
        return
      }
      if (ignoreLayoutChange.current) return
      ignoreLayoutChange.current = true
      onChildLayoutChanged?.(groupId, toServerLayouts(layouts))
    },
    [groupId, onChildLayoutChanged]
  )

  const onInnerDragStart = useCallback(() => {
    isDraggingRef.current = true
    pendingDragLayoutRef.current = null
    setChildOutside(false)
    // Suppress text selection across the page while dragging a child panel.
    document.body.classList.add('select-none')
  }, [])

  // Mirror the root grid's hover behavior: when the child has been dragged
  // outside its current group's body, highlight whichever other group's body
  // (if any) is under the cursor.
  const onInnerDrag = useCallback(
    (_l: any, _o: any, _n: any, _p: any, e: MouseEvent) => {
      onUserActionStart()
      const rect = bodyNodeRef.current?.getBoundingClientRect()
      if (!rect || !e) return
      const outside =
        e.clientX < rect.left ||
        e.clientX > rect.right ||
        e.clientY < rect.top ||
        e.clientY > rect.bottom
      setChildOutside(outside)
      if (!outside) {
        setHoverTarget('')
        setChildDragOut(null)
        return
      }
      const target = findGroupAtPoint(
        dropZonesRef.current,
        e.clientX,
        e.clientY
      )
      if (target && target !== groupId) {
        setHoverTarget(target)
        setChildDragOut(null)
      } else {
        setHoverTarget('')
        // Outside the source group AND not over any other group → dragging
        // toward the dashboard root. Publish the live cursor position so
        // Dashboard can render the drop indicator.
        setChildDragOut({ clientX: e.clientX, clientY: e.clientY })
      }
    },
    [groupId, onUserActionStart, setChildDragOut, setHoverTarget]
  )

  // When a child panel is dropped outside this Group's body, transfer it: to
  // another Group if the cursor is over one, otherwise back to the root grid.
  // Otherwise commit the pending inner layout (intra-group rearrange).
  const onInnerDragStop = useCallback(
    (
      _layout: any,
      _old: any,
      newItem: any,
      _placeholder: any,
      e: MouseEvent
    ) => {
      isDraggingRef.current = false
      setChildOutside(false)
      setHoverTarget('')
      setChildDragOut(null)
      document.body.classList.remove('select-none')
      const pending = pendingDragLayoutRef.current
      pendingDragLayoutRef.current = null
      if (!newItem?.i) return

      const rect = bodyNodeRef.current?.getBoundingClientRect()
      const outside =
        !!rect &&
        (e.clientX < rect.left ||
          e.clientX > rect.right ||
          e.clientY < rect.top ||
          e.clientY > rect.bottom)
      if (outside) {
        ignoreLayoutChange.current = true
        const target = findGroupAtPoint(
          dropZonesRef.current,
          e.clientX,
          e.clientY
        )
        onMoveChildOut?.(
          newItem.i,
          groupId,
          target && target !== groupId ? target : ''
        )
        return
      }
      if (pending) {
        ignoreLayoutChange.current = true
        onChildLayoutChanged?.(groupId, pending)
      }
    },
    [
      groupId,
      onChildLayoutChanged,
      onMoveChildOut,
      setChildDragOut,
      setHoverTarget
    ]
  )

  const onCollapseClick = useCallback(() => {
    // Reset the ignore flag so that when the body re-appears, any spurious
    // onLayoutChange fired by react-grid-layout on un-hide is suppressed.
    ignoreLayoutChange.current = true
    const next = !collapsed
    setCollapsed(next) // instant local update — body shows/hides immediately
    onCollapseToggle?.(groupId, next) // persist + resize outer grid in the background
  }, [collapsed, groupId, onCollapseToggle])

  const commitConfig = useCallback(
    (next: {
      title: string
      style: GroupStyleLike
      highlightColor: string
    }) => {
      const patch: {
        title?: string
        style?: GroupStyleLike
        highlightColor?: string
      } = {}
      if (next.title !== title) {
        setTitle(next.title)
        patch.title = next.title
      }
      if (next.style !== style) {
        setStyle(next.style)
        patch.style = next.style
      }
      if (next.highlightColor !== highlight) {
        setHighlight(next.highlightColor)
        patch.highlightColor = next.highlightColor
      }
      if (Object.keys(patch).length > 0) onConfigChange?.(groupId, patch)
    },
    [groupId, highlight, onConfigChange, style, title]
  )

  return (
    <div
      className={classNames(
        'bg-default-bg flex h-full w-full flex-col rounded-lg border transition-colors',
        isHoverTarget
          ? 'border-primary-500 ring-primary-500/40 ring-2'
          : 'border-border-color'
      )}
    >
      <div
        className={classNames(
          'group flex items-center justify-between gap-2 px-2 py-1',
          // Emphasis keeps rounded top corners so the tint doesn't bleed past
          // the panel's border-radius.
          isStyled && 'rounded-t-lg',
          isStyled && collapsed && 'rounded-b-lg',
          'group/group-panel'
        )}
        style={{ height: HEADER_HEIGHT_PX, ...headerStyle }}
      >
        <button
          type="button"
          className={classNames(
            'nonDraggable inline-flex h-6 w-6 items-center justify-center rounded',
            !allowEdit && 'invisible',
            isStyled
              ? 'hover:bg-black/10 dark:hover:bg-white/10'
              : 'hover:bg-hover text-text-foreground-secondary hover:text-text-foreground'
          )}
          style={isStyled ? { color: headerStyle.color } : undefined}
          onClick={onCollapseClick}
          disabled={!allowEdit}
          aria-label={collapsed ? 'Expand group' : 'Collapse group'}
        >
          {collapsed ? (
            <LuChevronRight className="h-4 w-4" />
          ) : (
            <LuChevronDown className="h-4 w-4" />
          )}
        </button>
        <h3
          className={classNames(
            'draggableHandle flex-1 cursor-move select-none truncate text-sm font-medium',
            centerTitle && 'text-center',
            isStyled ? '' : 'text-text-foreground'
          )}
          style={isStyled ? { color: headerStyle.color } : undefined}
        >
          {title}
        </h3>
        {allowEdit && (
          <div className="nonDraggable invisible flex items-center gap-1 group-hover/group-panel:visible">
            <button
              type="button"
              className={classNames(
                'inline-flex h-6 w-6 items-center justify-center rounded',
                isStyled
                  ? 'hover:bg-black/10 dark:hover:bg-white/10'
                  : 'hover:bg-hover text-text-foreground-secondary hover:text-text-foreground'
              )}
              style={isStyled ? { color: headerStyle.color } : undefined}
              onClick={() => setEditDialogOpen(true)}
              aria-label="Edit group"
            >
              <LuPencil className="h-3.5 w-3.5" />
            </button>
            <button
              type="button"
              className={classNames(
                'inline-flex h-6 w-6 items-center justify-center rounded',
                isStyled
                  ? 'hover:bg-black/10 dark:hover:bg-white/10'
                  : 'hover:bg-hover text-text-foreground-secondary hover:text-red-500'
              )}
              style={isStyled ? { color: headerStyle.color } : undefined}
              onClick={() => onRemoveGroup?.(groupId)}
              aria-label="Delete group"
            >
              <LuTrash2 className="h-3.5 w-3.5" />
            </button>
          </div>
        )}
      </div>
      {allowEdit && (
        <EditGroupDialog
          open={editDialogOpen}
          onClose={() => setEditDialogOpen(false)}
          title={title}
          style={style}
          highlightColor={highlight}
          onSave={commitConfig}
        />
      )}
      {/* Always keep this div (and the inner Responsive grid) mounted.
          Unmounting on collapse forces every child panel to destroy and
          re-init its chart/data on every expand — which is the source of lag.
          Instead we hide via display:none so the DOM subtree stays alive. */}
      <div
        className="nonDraggable relative flex-1 overflow-auto"
        ref={bodyNodeRef}
        style={collapsed ? { display: 'none' } : undefined}
      >
        {sortedChildren.length === 0 ? (
          <div className="text-text-foreground-secondary flex h-full items-center justify-center text-sm">
            {allowEdit
              ? 'Drag a panel here to add it to this group'
              : 'Empty group'}
          </div>
        ) : (
          <Responsive
            width={effectiveBodyWidth}
            // When the dragged child has left the body, hide this inner grid's
            // placeholder — the root (or target-group) indicator takes over.
            className={classNames(
              'layout',
              childOutside && 'rgl-hide-placeholder'
            )}
            cols={innerCols}
            isDraggable={!!allowEdit}
            isResizable={!!allowEdit}
            onLayoutChange={onInnerLayoutChange}
            onDragStart={onInnerDragStart}
            onDrag={onInnerDrag}
            onDragStop={onInnerDragStop}
            // Suppress text selection across the page during resize of a child.
            onResizeStart={() => {
              document.body.classList.add('select-none')
              onUserActionStart()
            }}
            onResize={onUserActionStart}
            onResizeStop={() => document.body.classList.remove('select-none')}
            layouts={childLayouts as any}
            {...InnerGridProps}
            // Default vertical compaction inside the group. We do NOT freeze
            // the inner grid during drag (like we do at the root) — the
            // dragged child needs to be able to move to whatever cell the
            // cursor reaches, so onInnerDragStop's outside-the-body check
            // can fire and reparent reliably.
            compactType="vertical"
          >
            {sortedChildren.map((child) => (
              <div data-testid={child.id} key={child.id} className="flex">
                {renderChild(child)}
              </div>
            ))}
          </Responsive>
        )}
      </div>
    </div>
  )
}
