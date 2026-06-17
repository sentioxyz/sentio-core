import { useMemo, useState } from 'react'
import { produce } from 'immer'
import { isEqual, sortBy, sortedUniqBy } from 'lodash'
import { LuCheck } from 'react-icons/lu'
import { VscRegex } from 'react-icons/vsc'
import { NewMultipleSelect, classNames } from '@sentio/ui-core'
import type { MetricInfoLike, QueryLike } from '../types/metrics'
import type { TemplateVariableLike } from '../types/dashboard'
import { SystemLabels } from './labels'
import { useLabelSearch } from './LabelSearchContext'

interface Props {
  metric?: MetricInfoLike
  value: QueryLike
  onChange: (value: QueryLike) => void
  variables?: { [p: string]: TemplateVariableLike }
  small?: boolean
  useRegex?: boolean
}

type LabelSelector = { display: string; key: string; value: string }

export function LabelsInput({
  value,
  metric,
  variables,
  onChange,
  small,
  useRegex
}: Props) {
  const [input, setInput] = useState('')
  const onSelectLabel = (labels: LabelSelector[]) => {
    const selector: { [key: string]: string } = {}
    labels.forEach((label) => {
      selector[label.key] = label.value
    })
    onChange(
      produce(value, (draft) => {
        draft.labelSelector = selector
      })
    )
  }
  const { setLabelSearchQuery } = useLabelSearch()

  const labelSelectors = useMemo(() => {
    const result: LabelSelector[] = []
    if (metric) {
      Object.entries(variables || {}).forEach(([name, variable]) => {
        const varname = `$${name}`
        const labelSelector = {
          display:
            variable.field == name ? varname : `${variable.field}: ${varname}`,
          key: variable.field!,
          value: `${varname}`
        }
        if (metric.labels && metric.labels[variable.field!]) {
          result.push(labelSelector)
        } else if (
          variable?.field &&
          SystemLabels.map((l) => l.name).includes(variable?.field)
        ) {
          result.push(labelSelector)
        }
      })

      for (const sl of SystemLabels) {
        sl.getValues(metric).forEach(({ value, display }) => {
          result.push({
            display: `${sl.name}: ${display}`,
            key: sl.field,
            value: value
          })
        })
      }
      let inputLabel = ''
      let inputValue = ''
      if (input.includes(':')) {
        ;[inputLabel, inputValue] = input.split(':')
        inputLabel = inputLabel.trim()
        inputValue = inputValue.trim()
      } else {
        inputValue = input.trim()
      }
      Object.entries(metric?.labels || {}).forEach(([key, values]) => {
        ;(values.values || []).forEach((value) => {
          result.push({
            display: `${key}:${value}`,
            key,
            value
          })
        })
        if (
          !useRegex ||
          (inputValue && key.includes(inputLabel) === false) ||
          !inputValue
        ) {
          return
        }
        result.push({
          display: `${key}: <contains> ${inputValue}`,
          key,
          value: JSON.stringify({
            operator: 'contains',
            value: inputValue,
            ignoreCase: true
          })
        })
      })
    }
    return sortedUniqBy(
      sortBy(result, (r) => r.display),
      (r) => r.display
    )
  }, [metric, variables, input, useRegex])

  const selectedLabels = useMemo(() => {
    const selector = value?.labelSelector || {}
    return Object.entries(selector).map(([key, value]) => {
      return (
        labelSelectors.find((ls) => ls.key == key && ls.value == value) || {
          display: `${key}:${value}`,
          key,
          value
        }
      )
    })
  }, [value?.labelSelector, labelSelectors])

  return (
    <NewMultipleSelect<LabelSelector>
      input={input}
      onInputChange={setInput}
      className={classNames(
        'border-main flex grow overflow-auto rounded-r-md border',
        small ? 'min-h-6' : 'min-h-8'
      )}
      options={labelSelectors}
      value={selectedLabels}
      onChange={onSelectLabel}
      displayFn={(o) => {
        const { display, value } = o
        const isRegex = /^\{.*\}$/.test(value)
        if (isRegex) {
          const valueObj = JSON.parse(value)
          return `${o.key}:<${valueObj?.opertaor ?? 'contains'}> ${valueObj?.value ?? value}`
        }
        return display
      }}
      disabled={!value.query}
      unSelectedText="(everywhere)"
      maxInputSize={30}
      displayIcon={(o: LabelSelector) => {
        const isRegex = /^\{.*\}$/.test(o.value)
        return isRegex ? (
          <VscRegex className="mr-1 inline-block h-3 w-3 align-top" />
        ) : null
      }}
      renderOption={(v: LabelSelector, _active: boolean, selected: boolean) => {
        const text = v.display
        const isRegex = /^\{.*\}$/.test(v.value)
        const title = `${text} ${isRegex ? ' (case-sensitive regex matcher)' : ''}`
        return (
          <>
            <span
              title={title}
              className={classNames(
                'block truncate',
                selected && 'font-medium'
              )}
            >
              {isRegex && (
                <VscRegex className="mr-1 inline-block h-3 w-3 align-top" />
              )}
              {text}
            </span>

            {selected && (
              <span
                className={classNames(
                  'absolute inset-y-0 right-0 flex items-center pr-4'
                )}
              >
                <LuCheck className="h-4 w-4" aria-hidden="true" />
              </span>
            )}
          </>
        )
      }}
      filterFn={(option: LabelSelector, input: string) => {
        const { display, value } = option
        const isRegex = /^\{.*\}$/.test(value)
        if (isRegex) {
          return true
        }
        return display.toLowerCase().includes(input.toLowerCase())
      }}
      validateFn={(option: LabelSelector) => {
        const isRegex = /^\{.*\}$/.test(option.value)
        if (isRegex) {
          return true
        }
        return labelSelectors.some((o) => isEqual(o, option))
      }}
      onFilterTextChange={setLabelSearchQuery}
    />
  )
}
