import { useEffect, useMemo, useRef, useState } from 'react'
import { BaseDialog, Select, classNames } from '@sentio/ui-core'
import { LuCheck } from 'react-icons/lu'
import type { GroupStyleLike } from '../types/enums'
import { useDarkMode } from '../utils/use-dark-mode'
import {
  DEFAULT_HIGHLIGHT_KEY,
  HIGHLIGHT_COLORS,
  resolveHighlight
} from './group-styles'

interface Props {
  open: boolean
  onClose: () => void
  // Existing values; the dialog seeds its draft state from them whenever it
  // opens, so cancelling discards unsaved edits.
  title: string
  style: GroupStyleLike
  highlightColor: string
  onSave: (next: {
    title: string
    style: GroupStyleLike
    highlightColor: string
  }) => void
}

interface StyleCardProps {
  selected: boolean
  label: string
  onClick: () => void
  preview: React.ReactNode
}

function StyleCard({ selected, label, onClick, preview }: StyleCardProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={classNames(
        'flex flex-col items-stretch overflow-hidden rounded-lg border bg-white text-left transition-colors',
        'dark:bg-default-bg',
        selected
          ? 'border-primary-600 ring-primary-600/30 ring-3 shadow-sm'
          : 'border-main hover:border-primary-400'
      )}
    >
      <div className="bg-hover/40 flex h-28 items-center justify-center">
        {preview}
      </div>
      <div
        className={classNames(
          'border-main flex items-center justify-center gap-1.5 border-t px-2 py-2 text-sm',
          selected
            ? 'text-primary-600 font-semibold'
            : 'text-text-foreground font-medium'
        )}
      >
        {selected && <LuCheck className="h-3.5 w-3.5" />}
        {label}
      </div>
    </button>
  )
}

// Mini-previews — render the same visual treatment that resolveHeaderStyle
// applies on the real GroupPanel, so the card always matches the live result.
function DefaultPreview() {
  return (
    <div className="border-main bg-default-bg text-text-foreground flex h-16 w-32 items-center rounded border px-2 text-base">
      Title
    </div>
  )
}

function EmphasisPreview({
  color
}: {
  color: { solid: string; foreground: string }
}) {
  return (
    <div className="border-main bg-default-bg flex h-16 w-32 flex-col rounded border">
      <div
        className="flex h-9 items-center justify-center text-sm font-semibold"
        style={{ backgroundColor: color.solid, color: color.foreground }}
      >
        Title
      </div>
      <div className="flex-1" />
    </div>
  )
}

export function EditGroupDialog({
  open,
  onClose,
  title,
  style,
  highlightColor,
  onSave
}: Props) {
  const [draftTitle, setDraftTitle] = useState(title)
  const [draftStyle, setDraftStyle] = useState<GroupStyleLike>(style)
  const [draftColor, setDraftColor] = useState<string>(highlightColor)
  const titleRef = useRef<HTMLInputElement | null>(null)
  const isDark = useDarkMode()

  // Re-seed every time the dialog opens so cancelling leaves no residue.
  useEffect(() => {
    if (!open) return
    setDraftTitle(title)
    setDraftStyle(style || 'DEFAULT')
    // If the user is opening a styled group that never had a color set, seed
    // with the default so the preview isn't blank.
    setDraftColor(
      highlightColor ||
        (style && style !== 'DEFAULT' ? DEFAULT_HIGHLIGHT_KEY : '')
    )
  }, [open, title, style, highlightColor])

  const previewColor = useMemo(
    () => resolveHighlight(draftColor, isDark),
    [draftColor, isDark]
  )

  const onPickStyle = (next: GroupStyleLike) => {
    setDraftStyle(next)
    // Auto-seed a default color when moving into a styled mode for the first
    // time — otherwise the preview shows a blank tint.
    if (next !== 'DEFAULT' && !draftColor) {
      setDraftColor(DEFAULT_HIGHLIGHT_KEY)
    }
  }

  const onOk = () => {
    onSave({
      title: draftTitle.trim() || 'Group',
      style: draftStyle,
      // Persist '' for DEFAULT so we don't pollute the model with an unused
      // color when the user reverts.
      highlightColor:
        draftStyle === 'DEFAULT' ? '' : draftColor || DEFAULT_HIGHLIGHT_KEY
    })
    onClose()
  }

  const colorOptions = useMemo(
    () =>
      HIGHLIGHT_COLORS.map((c) => {
        const resolved = resolveHighlight(c.key, isDark)
        return {
          value: c.key,
          title: c.name,
          label: ({ selected }: { selected?: boolean }) => (
            <div className="flex w-full items-center gap-2.5 pr-2">
              <div
                className="flex h-5 w-5 items-center justify-center rounded text-xs font-bold"
                style={{
                  backgroundColor: resolved.solid,
                  color: resolved.foreground
                }}
              >
                T
              </div>
              <span className="flex-1 text-sm">{c.name}</span>
              {selected && <LuCheck className="text-primary-600 h-4 w-4" />}
            </div>
          )
        }
      }),
    [isDark]
  )

  return (
    <BaseDialog
      title="Edit Group"
      open={open}
      onClose={onClose}
      cancelText="Cancel"
      onCancel={onClose}
      onOk={onOk}
      okText="Save"
      panelClassName="sm:max-w-xl"
      initialFocus={titleRef}
    >
      <div className="text-text-foreground px-4 pb-2 pt-4">
        <h4 className="text-text-foreground mb-3 text-sm font-semibold">
          Display options
        </h4>

        <label className="text-text-foreground-secondary text-ilabel mb-1 block">
          Title
        </label>
        <input
          ref={titleRef}
          type="text"
          value={draftTitle}
          onChange={(e) => setDraftTitle(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              onOk()
            }
          }}
          className="focus:border-primary-600 focus:ring-primary-600/30 focus:ring-3 hover:border-primary-600 sm:text-ilabel border-main mb-4 block w-full rounded-md"
        />

        <div className="mb-4 grid grid-cols-2 gap-3">
          <StyleCard
            selected={draftStyle === 'DEFAULT'}
            label="Default"
            onClick={() => onPickStyle('DEFAULT')}
            preview={<DefaultPreview />}
          />
          <StyleCard
            selected={draftStyle === 'EMPHASIS'}
            label="Emphasis"
            onClick={() => onPickStyle('EMPHASIS')}
            preview={<EmphasisPreview color={previewColor} />}
          />
        </div>

        {draftStyle !== 'DEFAULT' && (
          <>
            <label className="text-text-foreground-secondary text-ilabel mb-1 block">
              Highlight Color
            </label>
            <Select
              value={draftColor || DEFAULT_HIGHLIGHT_KEY}
              onChange={(v) => setDraftColor(v as string)}
              options={colorOptions}
              size="md"
              asLayer
            />
          </>
        )}
      </div>
    </BaseDialog>
  )
}
