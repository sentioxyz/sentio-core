import { Fragment, useEffect, useState } from 'react'
import { Dialog, Transition } from '@headlessui/react'
import type { BeforeMount } from '@monaco-editor/react'
import { NewButtonGroup, ShellIcon } from '@sentio/ui-core'
import { TbTerminal, TbBrandNodejs } from 'react-icons/tb'
import { LuArrowRight } from 'react-icons/lu'
import omit from 'lodash/omit'
import { CodeBlockWithTitle } from '../common/CodeBlock'
import { generateCurlCode, generateNodeCode } from './code-utils'

export enum ExportType {
  CURL = 'curl',
  NODE = 'node'
}

interface Props {
  open: boolean
  onClose: () => void
  /** Request body to render; identifying keys are stripped unless `noOmit`. */
  payload: Record<string, unknown>
  /** API base (e.g. https://app.sentio.xyz). Resolved & injected by the consumer. */
  apiHost?: string
  apiUrl?: string
  headers?: Record<string, unknown>
  defaultType?: ExportType
  noOmit?: boolean
  /** Register the monaco theme before mount (injected by the consumer). */
  onBeforeMount?: BeforeMount
}

export function CurlDialog({
  open,
  onClose,
  payload,
  apiHost = '',
  apiUrl,
  headers,
  defaultType = ExportType.CURL,
  noOmit,
  onBeforeMount
}: Props) {
  const data = noOmit
    ? payload
    : omit(payload, ['projectSlug', 'projectOwner', 'projectId'])
  const [type, setType] = useState(defaultType)
  const [curlContent, setCurlContent] = useState('')
  const [curlContentWithApiKey, setCurlContentWithApiKey] = useState('')
  const [nodeContent, setNodeContent] = useState('')

  useEffect(() => {
    let path = apiUrl || ''
    path = path.startsWith('/api/') ? path.substring(4) : path
    const url = path.startsWith('/') ? `${apiHost}${path}` : path
    setCurlContent(generateCurlCode(url, data, headers))
    setCurlContentWithApiKey(
      generateCurlCode(url, data, headers, undefined, undefined, 'param')
    )
    setNodeContent(generateNodeCode(url, data, headers))
  }, [apiUrl, apiHost, data, headers])

  useEffect(() => {
    setType(defaultType)
  }, [defaultType])
  return (
    <Transition.Root show={open} as={Fragment}>
      <Dialog
        as="div"
        className="text-icontent relative z-10"
        onClose={onClose}
      >
        <Transition.Child
          as={Fragment}
          enter="ease-out duration-300"
          enterFrom="opacity-0"
          enterTo="opacity-100"
          leave="ease-in duration-200"
          leaveFrom="opacity-100"
          leaveTo="opacity-0"
        >
          <div className="fixed inset-0 bg-gray-200/40 transition-opacity dark:bg-gray-200/50" />
        </Transition.Child>

        <div className="fixed inset-0 z-10 overflow-y-auto">
          <div className="flex min-h-full items-end justify-center p-4 text-center sm:items-center sm:p-0">
            <Transition.Child
              as={Fragment}
              enter="ease-out duration-300"
              enterFrom="opacity-0 translate-y-4 sm:translate-y-0 sm:scale-95"
              enterTo="opacity-100 translate-y-0 sm:scale-100"
              leave="ease-in duration-200"
              leaveFrom="opacity-100 translate-y-0 sm:scale-100"
              leaveTo="opacity-0 translate-y-4 sm:translate-y-0 sm:scale-95"
            >
              <Dialog.Panel className="bg-default-bg relative transform overflow-hidden rounded-lg pb-4 pt-5 text-left shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-4xl">
                <Dialog.Title
                  as="h3"
                  className="text-text-foreground px-4 text-lg font-medium leading-6 sm:px-6"
                >
                  Export As Code Snippet
                </Dialog.Title>
                <div className="my-2 space-y-4 border-t px-4 pt-4 sm:px-6">
                  <div className="text-icontent bg-primary-50 flex w-full flex-wrap items-center justify-between gap-y-1 rounded-md px-4 py-2 font-medium">
                    <span>
                      Replace{' '}
                      <code className="text-primary-600 dark:text-primary-800">
                        &lt;API_KEY&gt;
                      </code>
                      {` `}with your real API key.
                    </span>
                    <a href="/profile/apikeys" target="_blank" rel="noreferrer">
                      <div className="border-primary-400 text-primary-600 dark:text-primary-800 dark:border-primary-600 hover:bg-primary-100 active:bg-primary-200 flex flex-row items-center justify-center gap-1 rounded-md border border-solid pb-[7px] pl-2.5 pr-2.5 pt-[7px]">
                        <div className="text-icontent text-left font-medium">
                          Create a new API Key
                        </div>
                        <LuArrowRight className="h-4 w-4" />
                      </div>
                    </a>
                  </div>
                  <div className="flex items-center gap-2">
                    <NewButtonGroup
                      value={type}
                      onChange={setType}
                      buttons={[
                        {
                          label: 'Shell',
                          value: ExportType.CURL,
                          icon: <ShellIcon className="mr-2 h-4 w-4" />
                        },
                        {
                          label: 'Node',
                          value: ExportType.NODE,
                          icon: <TbBrandNodejs className="mr-2 h-4 w-4" />
                        }
                      ]}
                    />
                  </div>
                  {type === ExportType.CURL && (
                    <>
                      <CodeBlockWithTitle
                        value={curlContent}
                        showLineNumbers
                        language="bash"
                        title="Shell (Auth with Header)"
                        icon={
                          <TbTerminal className="mr-2 inline-block h-4 w-4" />
                        }
                        onBeforeMount={onBeforeMount}
                      />
                      <CodeBlockWithTitle
                        value={curlContentWithApiKey}
                        showLineNumbers
                        language="bash"
                        title="Shell (Auth with Query Param)"
                        icon={
                          <TbTerminal className="mr-2 inline-block h-4 w-4" />
                        }
                        onBeforeMount={onBeforeMount}
                      />
                    </>
                  )}
                  {type === ExportType.NODE && (
                    <CodeBlockWithTitle
                      value={nodeContent}
                      showLineNumbers
                      language="typescript"
                      title="Nodejs"
                      icon={
                        <TbBrandNodejs className="mr-2 inline-block h-4 w-4" />
                      }
                      onBeforeMount={onBeforeMount}
                    />
                  )}
                </div>
              </Dialog.Panel>
            </Transition.Child>
          </div>
        </div>
      </Dialog>
    </Transition.Root>
  )
}
