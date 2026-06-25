import { useMemo } from 'react'
import { produce } from 'immer'
import { defaults } from 'lodash'
import { DisclosurePanel, classNames } from '@sentio/ui-core'
import type {
  CalculationLike,
  ColumnTypeLike,
  TableConfigLike,
  TableDataLike,
  ValueConfigLike
} from '../../types'
import { getColumnNameId } from '../table-utils'
import { ValueOptions } from './ValueOptions'
import { defaultConfig as defaultValueConfig } from './ValueControls'

interface Props {
  config?: TableConfigLike
  defaultOpen?: boolean
  onChange: (config: TableConfigLike) => void
  data?: TableDataLike
}

export const defaultConfig: TableConfigLike = {
  calculation: 'LAST',
  sortColumns: [],
  showColumns: undefined,
  columnWidths: {},
  columnOrders: [],
  showPlainData: false,
  calculations: {},
  valueConfigs: {}
}

export function getDefaultValueConfig(type?: ColumnTypeLike): ValueConfigLike {
  switch (type) {
    case 'NUMBER':
      return {
        ...defaultValueConfig,
        valueFormatter: 'NumberFormatter'
      }
    case 'TIME':
      return {
        ...defaultValueConfig,
        valueFormatter: 'DateFormatter'
      }
    default:
      return {
        ...defaultValueConfig,
        valueFormatter: 'StringFormatter'
      }
  }
}

const CalculationItems = [
  { label: 'All', value: 'ALL' },
  { label: 'Last', value: 'LAST' },
  { label: 'First', value: 'FIRST' },
  { label: 'Total', value: 'TOTAL' },
  { label: 'Mean', value: 'MEAN' },
  { label: 'Max', value: 'MAX' },
  { label: 'Min', value: 'MIN' }
]

export function TableControls({ config, defaultOpen, onChange, data }: Props) {
  config = defaults(config, defaultConfig)

  function onCalculationChange(col: string, cal: CalculationLike) {
    config &&
      onChange(
        produce(config, (draft) => {
          draft.calculations = draft.calculations || {}
          draft.calculations[col] = cal
        })
      )
  }

  function onValueConfigChange(col: string, valueConfig: ValueConfigLike) {
    config &&
      onChange(
        produce(config, (draft) => {
          draft.valueConfigs = draft.valueConfigs || {}
          draft.valueConfigs[col] = valueConfig
        })
      )
  }

  function onMapSeriesAsColumnsChange(e: React.ChangeEvent<HTMLInputElement>) {
    config &&
      onChange(
        produce(
          config,
          (draft) => void (draft.showPlainData = e.target.checked)
        )
      )
  }

  const calculations = useMemo(() => {
    if (!config?.showPlainData) {
      return CalculationItems.filter((item) => item.value !== 'ALL')
    }
    return CalculationItems
  }, [config?.showPlainData])

  const isSql = data?.result !== undefined

  const columns = useMemo(() => {
    if (config?.showPlainData) {
      return []
    }
    const map: { [k: string]: { name: string; type?: ColumnTypeLike } } = {}

    if (isSql) {
      const results = data?.result
      if (results) {
        for (const [name, type] of Object.entries(results?.columnTypes || {})) {
          map[name] = {
            name,
            type
          }
        }
      }
    } else {
      const results = data?.results
      for (const r of results || []) {
        for (const s of r?.matrix?.samples || []) {
          const { columnId, columnName } = getColumnNameId(
            s?.metric?.labels || {},
            r.alias,
            s.metric?.displayName
          )
          map[columnId] = {
            name: columnName
          }
        }
      }
    }
    return Object.keys(map)
      .sort()
      .map((k) => ({ columnId: k, column: map[k] }))
  }, [data, config])

  return (
    <DisclosurePanel
      title="Table Options"
      defaultOpen={defaultOpen}
      containerClassName="w-full bg-default-bg"
    >
      {!isSql && (
        <div className="mt-1 flex gap-4">
          <div
            className={classNames(
              'text-text-foreground mx-2 inline-flex items-center sm:text-sm'
            )}
          >
            <input
              type="checkbox"
              className="border-main mr-1 rounded-sm"
              checked={config?.showPlainData}
              onChange={onMapSeriesAsColumnsChange}
            />
            Show plain data
          </div>
          {config?.showPlainData && (
            <div className="flex">
              <span className="border-main inline-flex items-center rounded-l-md border  bg-gray-50 px-3  sm:text-sm">
                Calculation
              </span>
              <select
                value={config.calculation}
                className="border-main text-text-foreground-secondary inline-flex items-center rounded-r-md border  border-l-0  pl-4  pr-7 sm:text-sm"
                onChange={(e) =>
                  onCalculationChange('', e.target.value as CalculationLike)
                }
              >
                {calculations.map((d) => {
                  return (
                    <option key={d.value} value={d.value}>
                      {d.label}
                    </option>
                  )
                })}
              </select>
            </div>
          )}
          <div></div>
        </div>
      )}

      <div className="divide-border-color flex flex-col gap-2 divide-y">
        {columns.map(({ columnId, column }) => (
          <div className="flex items-start pb-2" key={columnId}>
            <h4 className="text-text-foreground w-48 px-2 text-sm font-medium leading-[30px]">
              {column.name}
            </h4>
            <div className="flex flex-1 flex-wrap items-start gap-x-2 rounded-md">
              {!isSql && (
                <div className="flex">
                  <span className="sm:text-ilabel border-main inline-flex items-center rounded-l-md  border bg-gray-50 px-3">
                    Calculation
                  </span>
                  <select
                    value={
                      (config?.calculations &&
                        config?.calculations[columnId]) ||
                      'LAST'
                    }
                    className="sm:text-ilabel border-main text-text-foreground inline-flex h-full items-center rounded-r-md border border-l-0 py-1.5 pl-4 pr-7"
                    onChange={(e) =>
                      onCalculationChange(
                        columnId,
                        e.target.value as CalculationLike
                      )
                    }
                  >
                    {calculations.map((d) => {
                      return (
                        <option key={d.value} value={d.value}>
                          {d.label}
                        </option>
                      )
                    })}
                  </select>
                </div>
              )}
              <ValueOptions
                onChange={(cfg) => onValueConfigChange(columnId, cfg)}
                config={
                  (config?.valueConfigs && config.valueConfigs[columnId]) ||
                  getDefaultValueConfig(column.type)
                }
              />
            </div>
          </div>
        ))}
      </div>
    </DisclosurePanel>
  )
}
