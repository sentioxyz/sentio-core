import { memo, useCallback, useMemo, useRef } from 'react'
import { PopupMenuButton, classNames } from '@sentio/ui-core'
import type { IMenuItem } from '@sentio/ui-core'
import { LuChevronDown } from 'react-icons/lu'
import { VscAdd } from 'react-icons/vsc'
import type { DashboardLike } from '../types/dashboard'

interface Props {
  dashboard?: DashboardLike
  dashboards?: DashboardLike[]
  allowEdit?: boolean
  onSelectDashboard: (dashboardId: string) => void
  onNewDashboard: () => void
}

const NEW_DASHBOARD_KEY = 'new-dashboard'

export const DashboardTitle = memo(function DashboardTitle({
  dashboard,
  dashboards,
  allowEdit,
  onSelectDashboard,
  onNewDashboard
}: Props) {
  const buttonRef = useRef<HTMLButtonElement>(null)

  const onSelect = useCallback(
    (dashboardId: string) => {
      if (dashboardId === NEW_DASHBOARD_KEY) {
        onNewDashboard()
        return
      }
      onSelectDashboard(dashboardId)
    },
    [onNewDashboard, onSelectDashboard]
  )

  const items = useMemo(() => {
    if (!dashboards) {
      return []
    }
    const list = dashboards.map((d) => ({
      key: d.id as string,
      label: d.name as string
    }))
    list.sort((a, b) => a.label.localeCompare(b.label))
    const items: IMenuItem[][] = [list]
    if (allowEdit) {
      items.push([
        {
          key: NEW_DASHBOARD_KEY,
          label: 'New Dashboard',
          icon: <VscAdd className="mr-2" />
        }
      ])
    }
    return items
  }, [dashboards, allowEdit])

  return (
    <PopupMenuButton
      onSelect={onSelect}
      items={items}
      width="180px"
      selectedKey={dashboard?.id}
      offset={{
        crossAxis: -2
      }}
      buttonIcon={(open) => (
        <button
          className={classNames(
            'ring-primary group -mx-1.5 inline-flex items-center gap-2 rounded-sm px-1.5',
            open
              ? 'dark:bg-primary-600 ring-1 dark:ring-0'
              : 'dark:hover:bg-primary-600 hover:outline-hidden hover:ring-1 dark:hover:ring-0'
          )}
          ref={buttonRef}
          aria-label="Dashboard title"
        >
          <h1
            data-testid="dashboard-title"
            className={classNames(
              'text-ititle dark:group-hover:text-text-foreground group-hover:text-primary-500 group-active:text-primary-400 inline-block max-w-xs truncate font-semibold',
              open
                ? 'text-primary-500 dark:text-inherit'
                : 'text-text-foreground'
            )}
          >
            {dashboard?.name ?? 'Dashboard'}
          </h1>
          <LuChevronDown className="icon group-hover:text-primary-500 dark:group-hover:text-text-foreground group-active:text-primary-400 inline-block" />
        </button>
      )}
    />
  )
})
