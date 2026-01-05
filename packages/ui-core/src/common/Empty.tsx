import React, { useContext } from 'react'
import { SvgFolderContext } from '../utils/extension-context'

interface EmptyProps {
  src?: string
  title?: React.ReactNode
}

export const Empty: React.FC<EmptyProps> = (props) => {
  const parentFolder = useContext(SvgFolderContext)
  return (
    <div className="mx-auto w-fit">
      <img
        src={props.src ?? `${parentFolder}/empty.svg`}
        width={88}
        height={88}
        alt="empty icon"
        className="mx-auto"
      />
      <span className="text-ilabel text-gray">
        {props.title || 'No results found'}
      </span>
    </div>
  )
}
