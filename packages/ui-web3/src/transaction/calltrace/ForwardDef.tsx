import { memo, MutableRefObject } from 'react'
import { Location } from '@sentio/debugger'
import { ArrowUturnRightIcon } from '@heroicons/react/24/outline'
import { LocationWithInstructionIndex } from '@sentio/debugger-common'

interface ForwardDefProps {
  name?: string
  location?: LocationWithInstructionIndex
  onClick?: (location: LocationWithInstructionIndex) => void
}

export const ForwardDef = memo(function ForwardDefMemo({
  name,
  location,
  onClick
}: ForwardDefProps) {
  if (location === undefined) {
    return null
  }
  return (
    <div className="!mt-6">
      <button
        className="text-gray hover:border-primary hover:text-primary cursor-pointer rounded border border-gray-300  px-2 py-1"
        onClick={() => {
          if (location) {
            onClick?.(location)
          }
        }}
      >
        <ArrowUturnRightIcon className="mr-2 inline-block h-3.5 w-3.5" />
        Go to {name} Implementation
      </button>
    </div>
  )
})
