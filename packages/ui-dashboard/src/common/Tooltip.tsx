import { useEffect } from 'react'
import { useFloating, FloatingPortal, shift } from '@floating-ui/react'

interface Props {
  referenceElement?: HTMLElement
  text: string
}

export function Tooltip({ referenceElement, text }: Props) {
  const { x, y, refs, strategy } = useFloating({
    placement: 'bottom',
    middleware: [shift()]
  })

  useEffect(() => {
    if (referenceElement) refs.setReference(referenceElement)
  }, [refs, referenceElement])

  if (!referenceElement || !text) {
    return null
  }

  return (
    <FloatingPortal>
      <div
        ref={refs.setFloating}
        className="z-tooltip pointer-events-none rounded-md bg-black/70 px-2 py-1 text-white backdrop-opacity-60"
        style={{
          position: strategy,
          top: y ?? 0,
          left: x ?? 0
        }}
      >
        {text}
      </div>
    </FloatingPortal>
  )
}
