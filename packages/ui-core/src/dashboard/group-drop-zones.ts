import { atom } from 'jotai'

// Height of one row in the dashboard's outer react-grid-layout. Shared so that
// a collapsed Group (outer h = 1) is exactly the same height as the Group's
// header row.
export const ROOT_ROW_HEIGHT = 60

// A registered Group's body region — used to hit-test a drag-stop point and
// decide whether the dropped panel should be transferred into that Group.
export type GroupDropZone = {
  id: string
  getRect: () => DOMRect | null
}

export const groupDropZonesAtom = atom<Record<string, GroupDropZone>>({})

// While a panel is being dragged, holds the id of the Group whose body the
// cursor is currently over (empty string when not over any). GroupPanel reads
// this to render a hover highlight.
export const groupHoverTargetAtom = atom<string>('')

// While a Group's CHILD panel is being dragged with the cursor outside the
// source group AND not over any other group (i.e. the user is dragging it out
// to the dashboard root), holds the live cursor position. Dashboard reads this
// to render a drop-target indicator at the snapped root grid cell — and to
// resolve the destination cell when the panel is dropped.
export const childDragOutAtom = atom<{
  clientX: number
  clientY: number
} | null>(null)

export function findGroupAtPoint(
  zones: Record<string, GroupDropZone>,
  clientX: number,
  clientY: number
): string | null {
  for (const zone of Object.values(zones)) {
    const rect = zone.getRect()
    if (!rect) continue
    if (
      clientX >= rect.left &&
      clientX <= rect.right &&
      clientY >= rect.top &&
      clientY <= rect.bottom
    ) {
      return zone.id
    }
  }
  return null
}
