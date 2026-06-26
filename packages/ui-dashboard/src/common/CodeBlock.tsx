import MonacoEditor from '@monaco-editor/react'
import type { BeforeMount } from '@monaco-editor/react'
import { ReactNode, useState } from 'react'
import { classNames, CopyButton, useDarkMode } from '@sentio/ui-core'
import { LuChevronDown } from 'react-icons/lu'

export type SupportedLanguages =
  | 'bash'
  | 'go'
  | 'javascript'
  | 'json'
  | 'python'
  | 'rust'
  | 'sql'
  | 'tsx'
  | 'typescript'
  | string

// Monaco uses different language identifiers for some languages
const LANGUAGE_MAP: Record<string, string> = {
  bash: 'shell',
  jsx: 'javascript',
  tsx: 'typescript'
}

function toMonacoLanguage(lang: string): string {
  return LANGUAGE_MAP[lang] ?? lang
}

const BASE_OPTIONS = {
  readOnly: true,
  scrollBeyondLastLine: false,
  minimap: { enabled: false },
  renderLineHighlight: 'none' as const,
  folding: false
} as const

interface Props {
  value: string
  language: SupportedLanguages
  showLineNumbers?: boolean
  maxHeight?: string
  className?: string
  /** Register the editor theme (sentio / sentio-dark) before mount. Injected by the consumer. */
  onBeforeMount?: BeforeMount
}

export const CodeBlockWithTitle = ({
  title,
  icon,
  value,
  language,
  showLineNumbers,
  maxHeight = '300px',
  className,
  onBeforeMount
}: Props & {
  title: string
  icon?: ReactNode
}) => {
  const isDarkMode = useDarkMode()
  const [collapsed, setCollapsed] = useState(false)
  return (
    <div
      className={classNames(
        'text-icontent border-main overflow-hidden rounded-lg border',
        className
      )}
    >
      <div
        className="flex w-full cursor-pointer items-center justify-between border-b bg-gray-200 p-2"
        onClick={() => setCollapsed((c) => !c)}
      >
        <span className="text-ilabel text-text-foreground font-medium">
          {icon}
          {title}
        </span>
        <div className="flex items-center gap-1">
          <div onClick={(e) => e.stopPropagation()}>
            <CopyButton text={value} size={16} />
          </div>
          <LuChevronDown
            className={classNames(
              'text-text-foreground-secondary h-4 w-4 transition-transform duration-200',
              collapsed ? '-rotate-90' : ''
            )}
          />
        </div>
      </div>
      {!collapsed && (
        <MonacoEditor
          theme={isDarkMode ? 'sentio-dark' : 'sentio'}
          language={toMonacoLanguage(language)}
          value={value}
          height={maxHeight}
          beforeMount={onBeforeMount}
          options={{
            ...BASE_OPTIONS,
            lineNumbers: showLineNumbers ? 'on' : 'off'
          }}
        />
      )}
    </div>
  )
}
