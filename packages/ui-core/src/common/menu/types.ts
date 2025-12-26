import { ReactNode } from 'react'

export interface IMenuItem {
  key: string
  label: ReactNode
  icon?: ReactNode
  status?: string
  disabled?: boolean
  disabledHint?: ReactNode
  items?: IMenuItem[][] // nested menu
  data?: any // extra data
}

export type OnSelectMenuItem = (
  key: string,
  event: React.MouseEvent,
  data?: any
) => void
