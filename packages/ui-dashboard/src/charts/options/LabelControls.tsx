import { useEffect, useMemo } from 'react'
import { produce } from 'immer'
import { Button as NewButton, DisclosurePanel, HelpIcon } from '@sentio/ui-core'
import type { LabelConfigLike } from '../../types'

interface Props {
  config?: LabelConfigLike
  setConfig: (value: LabelConfigLike) => void
  defaultOpen?: boolean
}

const initialConfig: LabelConfigLike = {
  columns: [],
  alias: ''
}

export const LabelControls = ({ config, setConfig, defaultOpen }: Props) => {
  // Migrate existing columns config to alias on component mount
  useEffect(() => {
    if (config?.columns && config.columns.length > 0 && !config.alias) {
      const aliasParts: string[] = []
      config.columns.forEach((colConfig) => {
        if (!colConfig.name) return // Skip if name is undefined

        if (colConfig.showLabel === false && colConfig.showValue === false) {
          // ignore
        } else if (colConfig.showValue === false) {
          aliasParts.push(colConfig.name)
        } else {
          aliasParts.push(`{{${colConfig.name}}}`)
        }
      })

      if (aliasParts.length > 0) {
        const migratedAlias = aliasParts.join(', ')
        setConfig(
          produce(config, (draft) => {
            draft.alias = migratedAlias
            draft.columns = [] // Clear the old columns config
          })
        )
      }
    }
  }, [config, setConfig])

  const onAliasChanged = (alias: string) => {
    setConfig(
      produce(config ?? initialConfig, (draft) => {
        draft.alias = alias
      })
    )
  }

  const _defaultOpen = useMemo(() => {
    if (defaultOpen) {
      return true
    }
    return (
      config?.alias !== '' || (config?.columns && config.columns.length > 0)
    )
  }, [config, defaultOpen])

  return (
    <DisclosurePanel
      title="Label Controls"
      defaultOpen={_defaultOpen}
      containerClassName="w-full bg-default-bg"
    >
      <div className="flex items-center gap-2">
        <div className="inline-flex h-8">
          <span className="sm:text-icontent border-main inline-flex items-center rounded-l-md border border-r-0 bg-gray-50 px-2 font-medium">
            Label Alias
            <HelpIcon
              text={
                <div className="text-icontent text-text-foreground">
                  <div>Series name override or template.</div>
                  <div>
                    Ex.{' '}
                    <span className="text-primary mx-1 font-semibold italic">
                      {'{{contract}}'}
                    </span>{' '}
                    will be replaced with the value of the contract label.
                  </div>
                </div>
              }
            />
          </span>
          <input
            type="text"
            value={config?.alias || ''}
            onChange={(e) => onAliasChanged(e.target.value)}
            placeholder="Enter alias..."
            className="focus:border-primary-500 sm:text-icontent border-main inline-flex w-64 items-center rounded-r-md border px-2"
          />
        </div>
        <NewButton
          type="button"
          role="link"
          onClick={() => {
            setConfig(initialConfig)
          }}
        >
          Reset
        </NewButton>
      </div>
    </DisclosurePanel>
  )
}
