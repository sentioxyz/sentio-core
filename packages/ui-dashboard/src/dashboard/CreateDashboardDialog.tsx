import { useCallback, useReducer, useRef, useState } from 'react'
import { BaseDialog, Button, Select } from '@sentio/ui-core'
import { LuX } from 'react-icons/lu'
import type { DashboardLike } from '../types/dashboard'
import type { DashboardVisibilityLike } from '../types/enums'

interface Props {
  open: boolean
  onClose: () => void
  onCreate: (data: DashboardLike) => Promise<void>
  // pre-computed default title (depends on app user/clock) injected by the wrapper
  defaultName: string
  projectId?: string
  ownerId?: string
  showExternal?: boolean
}

// ponytail: inlined text-to-color (single use) — deterministic tag chip color
function textToColor(text: string): string {
  let hash = 5381
  for (let i = 0; i < text.length; i++) {
    hash = (hash * 33) ^ text.charCodeAt(i)
  }
  const r = (hash >> 16) & 0xff
  const g = (hash >> 8) & 0xff
  const b = hash & 0xff
  const color = `#${((r << 16) | (g << 8) | b).toString(16).padStart(6, '0')}`
  const grayscale = Math.round(0.299 * r + 0.587 * g + 0.114 * b)
  return grayscale < 128 ? color : '#000000'
}

type TagAction = { type: 'add' | 'remove'; payload: string } | { type: 'reset' }

function tagsReducer(state: string[], action: TagAction): string[] {
  if (action.type === 'add') {
    return [...state, action.payload]
  }
  if (action.type === 'remove') {
    return state.filter((item) => item !== action.payload)
  }
  return []
}

export const CreateDashboardDialog = ({
  open,
  onClose,
  onCreate,
  defaultName,
  projectId,
  ownerId,
  showExternal
}: Props) => {
  const [name, setName] = useState('')
  const [visibility, setVisiblity] = useState<DashboardVisibilityLike>(
    showExternal ? 'PUBLIC' : 'INTERNAL'
  )
  const [tags, dispatchTagAction] = useReducer(tagsReducer, [])
  const [processing, setProcessing] = useState(false)
  const inputElementRef = useRef<HTMLInputElement>(null)
  const tagInputElementRef = useRef<HTMLInputElement>(null)
  const resetForm = useCallback(() => {
    setName('')
    setVisiblity(showExternal ? 'PUBLIC' : 'INTERNAL')
    dispatchTagAction({ type: 'reset' })
  }, [showExternal])
  const onCloseAndReset = useCallback(() => {
    onClose?.()
    resetForm()
  }, [onClose, resetForm])
  const onOk = () => {
    setProcessing(true)
    onCreate({
      name: name || defaultName,
      projectId,
      tags,
      visibility,
      ownerId
    })
      .then(() => {
        resetForm()
      })
      .finally(() => {
        setProcessing(false)
      })
  }

  return (
    <BaseDialog
      title="Create Dashboard"
      open={open}
      onClose={onCloseAndReset}
      cancelText="Close"
      onCancel={onCloseAndReset}
      onOk={onOk}
      okProps={{
        processing
      }}
      okText="Create"
      footerBorder={false}
      initialFocus={inputElementRef}
    >
      <form
        method="dialog"
        className="text-text-foreground relative mb-4 mt-2 px-4"
        onSubmit={onOk}
      >
        <div className="grid py-2 text-sm">
          <div className="sm:text-ilabel text-text-foreground-secondary mb-2 mt-1">
            Dashboard Name
          </div>
          <input
            placeholder={defaultName}
            type="text"
            required={true}
            name="dashboard-name"
            id="new-dashboard-name"
            onChange={(e) => setName(e.target.value)}
            value={name || ''}
            className="focus:border-primary-600 focus:ring-primary-600/30 focus:ring-3 hover:border-primary-600 sm:text-ilabel border-main block w-full rounded-md"
            ref={inputElementRef}
          />
        </div>
        {showExternal ? (
          <div className="py-4">
            <div className="grid grid-cols-12 items-start gap-4">
              <div className="sm:text-ilabel text-text-foreground-secondary col-span-2">
                Visibility
              </div>
              <div className="col-span-10">
                <Select
                  value={visibility}
                  onChange={(value) =>
                    setVisiblity(value as DashboardVisibilityLike)
                  }
                  options={[
                    {
                      label: 'Public',
                      value: 'PUBLIC'
                    },
                    {
                      label: 'Private',
                      value: 'PRIVATE'
                    }
                  ]}
                />
              </div>
              <div className="sm:text-ilabel text-text-foreground-secondary col-span-2">
                Tags
              </div>
              <div className="col-span-10 flex">
                <input
                  placeholder="Add a new tag"
                  type="text"
                  className="sm:text-ilabel shadow-xs border-main inline-block w-full  rounded-l-md"
                  ref={tagInputElementRef}
                  onKeyDown={(evt) => {
                    if (evt.key === 'Enter') {
                      evt.preventDefault()
                      if (!tagInputElementRef.current) return
                      const value = tagInputElementRef.current.value
                      if (value) {
                        dispatchTagAction({ type: 'add', payload: value })
                        tagInputElementRef.current.value = ''
                      }
                    }
                  }}
                />
                <Button
                  size="sm"
                  role="primary"
                  className="inline-block rounded-l-none"
                  onClick={() => {
                    if (!tagInputElementRef.current) return
                    const value = tagInputElementRef.current.value
                    if (value) {
                      dispatchTagAction({ type: 'add', payload: value })
                      tagInputElementRef.current.value = ''
                    }
                  }}
                >
                  Add
                </Button>
              </div>
              <div className="col-span-10 col-start-3 flex flex-wrap items-center gap-x-4 gap-y-2">
                {tags.map((item) => (
                  <span
                    className="text-icontent inline-flex rounded-sm px-2 py-1 font-medium text-white"
                    key={item}
                    style={{
                      backgroundColor: textToColor(item)
                    }}
                  >
                    <span>{item}</span>
                    <LuX
                      className="ml-2 h-4 w-4 cursor-pointer"
                      onClick={() => {
                        dispatchTagAction({ type: 'remove', payload: item })
                      }}
                    />
                  </span>
                ))}
              </div>
            </div>
          </div>
        ) : null}
      </form>
    </BaseDialog>
  )
}
