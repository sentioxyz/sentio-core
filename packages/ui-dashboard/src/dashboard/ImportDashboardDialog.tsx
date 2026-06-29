import { useCallback, useEffect, useState } from 'react'
import { useDropzone } from 'react-dropzone'
import MonacoEditor from '@monaco-editor/react'
import type { BeforeMount } from '@monaco-editor/react'
import {
  BaseDialog,
  Button,
  CopyButton,
  ImportIcon,
  classNames,
  useDarkMode
} from '@sentio/ui-core'
import { LuHardDriveUpload } from 'react-icons/lu'

interface Props {
  open: boolean
  onClose: () => void
  onImport: (json: string) => Promise<void>
  onBeforeMount?: BeforeMount
}

// ponytail: inlined isMac (single use) — only drives the paste-shortcut hint label
const isMac = () =>
  typeof window !== 'undefined' &&
  window.navigator.userAgent.toLowerCase().indexOf('mac') !== -1

export function ImportDashboardDialog({
  open,
  onClose,
  onImport,
  onBeforeMount
}: Props) {
  const isDarkMode = useDarkMode()
  const [json, setJson] = useState<string>('')

  const onImportClick = async () => {
    await onImport(json)
    onClose()
  }
  const onExitClose = useCallback(() => {
    onClose()
    setJson('')
  }, [onClose])

  const onDrop = useCallback((acceptedFiles: File[]) => {
    acceptedFiles.forEach((file) => {
      const reader = new FileReader()
      reader.onload = () => {
        setJson(reader.result as string)
      }
      reader.readAsText(file)
    })
  }, [])
  const {
    getRootProps,
    getInputProps,
    open: openFileSelect
  } = useDropzone({ onDrop, noClick: true })

  useEffect(() => {
    const handlePaste = (event: ClipboardEvent) => {
      setJson(event.clipboardData?.getData('Text') ?? '')
    }
    if (open) {
      window.addEventListener('paste', handlePaste)
    }
    return () => {
      window.removeEventListener('paste', handlePaste)
    }
  }, [open])

  return (
    <BaseDialog
      title="Import dashboard JSON"
      open={open}
      onClose={onExitClose}
      onOk={onImportClick}
      okText="Import"
      onCancel={onExitClose}
      cancelText="Close"
      footerBorder={false}
      extraButtons={
        <Button
          role="secondary"
          onClick={openFileSelect}
          className={classNames(
            'absolute left-4',
            json === '' ? 'hidden' : 'block'
          )}
          icon={<LuHardDriveUpload />}
          size="md"
        >
          Choose file
        </Button>
      }
    >
      <form className="relative">
        <div>
          {json !== '' ? (
            <div className="relative px-[18px] py-4">
              <div
                className="z-1 absolute right-10 top-8"
                onClick={(evt) => {
                  evt.stopPropagation()
                  evt.preventDefault()
                }}
              >
                <CopyButton text={json} size={16} />
              </div>
              <div className="focus-within:border-primary-300 h-[324px] overflow-hidden rounded-sm border">
                <MonacoEditor
                  value={json}
                  theme={isDarkMode ? 'sentio-dark' : 'sentio'}
                  language="json"
                  beforeMount={onBeforeMount}
                  onChange={(code) => {
                    setJson(code ?? '')
                  }}
                  options={{
                    minimap: {
                      enabled: false
                    },
                    lineNumbers: 'off'
                  }}
                />
              </div>
            </div>
          ) : null}
          <div
            className={classNames(
              'inset-x-px',
              json === '' ? 'block' : 'hidden'
            )}
          >
            <div className="flex w-full items-center space-x-3 px-[18px] py-4">
              <div className="flex h-[324px] w-full" {...getRootProps()}>
                <input {...getInputProps()} />
                <div className="focus:outline-hidden border-main hover:border-primary-600 relative m-auto grid h-full w-full place-items-center rounded-sm  border border-dashed p-5 text-center focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2">
                  <div className="flex flex-col items-center">
                    <div>
                      <ImportIcon className="text-text-foreground-disabled m-auto h-14 w-14" />
                    </div>
                    <div className="text-text-foreground text-ilabel mt-2 font-semibold">
                      Drag and drop or{' '}
                      <button
                        className="text-primary"
                        type="button"
                        onClick={openFileSelect}
                      >
                        browse
                      </button>
                    </div>
                    <div className="text-text-foreground-secondary text-icontent mt-1 space-x-1">
                      <span>or</span>
                      <span className="border-main rounded-sm border  px-1">
                        {isMac() ? '⌘ + V' : 'Ctrl + V'}
                      </span>
                      <span>to paste from clipboard</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </form>
    </BaseDialog>
  )
}
