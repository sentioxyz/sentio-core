import { useCallback, useEffect, useMemo, useRef } from 'react'
import { debounce } from 'lodash'
import { SlideOver, Button, useBoolean, classNames } from '@sentio/ui-core'
import { PiBracketsCurlyBold } from 'react-icons/pi'
import type { PanelLike } from '../types/dashboard'

const panelExample = `Paste the panel configuration here.
Here is an note type example:
  {
    "name": "",
    "chart": {
      "type": "NOTE",
      "queries": [],
      "formulas": [],
      "config": null,
      "note": {
        "content": "* asdasda",
        "fontSize": "MD",
        "textAlign": "LEFT"
      },
      "datasourceType": "NOTES",
      "segmentationQueries": [],
      "insightsQueries": [],
      "eventLogsConfig": null,
      "retentionQuery": null,
      "sqlQuery": ""
    }
  }
`

function isValidPanelData(data: string) {
  try {
    const parsed = JSON.parse(data)
    return parsed.name !== undefined || parsed.chart !== undefined
  } catch {
    return false
  }
}

export const ImportPanelDialog = ({
  show,
  onClose: _onClose,
  onSubmit
}: {
  show: boolean
  onClose: () => void
  onSubmit: (p: PanelLike) => Promise<void>
}) => {
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const {
    value: isInvalid,
    setTrue: setInvalid,
    setFalse: setValid
  } = useBoolean(false)

  useEffect(() => {
    if (show) {
      setTimeout(() => {
        textareaRef.current?.focus()
      }, 500)
    }
  }, [show])

  const debouncedValidate = useMemo(
    () =>
      debounce((value: string) => {
        if (isValidPanelData(value)) {
          setValid()
        } else {
          setInvalid()
        }
      }, 500),
    [setValid, setInvalid]
  )

  const handleSubmit = () => {
    if (isInvalid) return
    try {
      const parsed = JSON.parse(textareaRef.current?.value || '')
      onSubmit(parsed)
    } catch {
      setInvalid()
    }
  }

  const onClose = useCallback(() => {
    _onClose()
    setValid()
    if (textareaRef.current) {
      textareaRef.current.value = ''
    }
  }, [_onClose, setValid])

  return (
    <SlideOver
      title="Import Panel"
      open={show}
      onClose={onClose}
      size="lg"
      triggerClose="button"
    >
      <div className="w-full space-y-6 p-4">
        <textarea
          ref={textareaRef}
          className={classNames(
            'text-icontent text-text-foreground h-[60vh] w-full rounded-sm border',
            isInvalid ? 'border-rose-600! ring-rose-600!' : ''
          )}
          rows={10}
          onChange={(evt) => debouncedValidate(evt.target.value)}
          placeholder={panelExample}
        />
        <div className="flex w-full items-center justify-between">
          <span>
            <Button status="danger" size="lg" onClick={onClose}>
              Cancel
            </Button>
          </span>
          <span className="inline-flex gap-2">
            <Button
              size="lg"
              onClick={() => {
                try {
                  const parsed = JSON.parse(textareaRef.current?.value || '')
                  textareaRef.current!.value = JSON.stringify(parsed, null, 2)
                } catch {
                  setInvalid()
                }
              }}
              icon={<PiBracketsCurlyBold />}
            >
              Format
            </Button>
            <Button role="primary" size="lg" onClick={handleSubmit}>
              Submit
            </Button>
          </span>
        </div>
      </div>
    </SlideOver>
  )
}
