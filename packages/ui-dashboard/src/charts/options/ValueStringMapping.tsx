import { LuPlus, LuTrash2 } from 'react-icons/lu'
import { Button, classNames } from '@sentio/ui-core'
import { produce } from 'immer'
import type { MappingRuleLike } from '../../types'

const operators = {
  '>': 'greater than',
  '>=': 'greater or equal',
  '==': 'equal',
  '!=': 'not equal',
  '<': 'less than',
  '<=': 'less or equal'
}

interface Props {
  rules: MappingRuleLike[]
  onChange: (rules: MappingRuleLike[]) => void
}

const renderTreeLine = (index: number, isLast: boolean) => {
  return (
    <div className="mr-2 flex w-3 flex-col items-center justify-center">
      <div className="flex h-full w-full items-center">
        <div
          className={classNames(
            'w-px bg-gray-300',
            isLast
              ? 'h-1/2 self-start'
              : index === 0
                ? 'h-full self-end'
                : 'h-full'
          )}
        ></div>
        <div className="h-px w-3 bg-gray-300"></div>
      </div>
    </div>
  )
}

export function ValueStringMapping({ rules, onChange }: Props) {
  const addRule = () => {
    onChange(
      produce(rules, (draft) => {
        draft = draft || []
        draft.push({
          comparison: '==',
          value: 0,
          text: ''
        })
      })
    )
  }

  function removeRule(index: number) {
    onChange(
      produce(rules, (draft) => {
        draft.splice(index, 1)
      })
    )
  }

  function changeRule(index: number, field: string, value: any) {
    onChange(
      produce(rules, (draft) => {
        ;(draft[index] as Record<string, any>)[field] = value
      })
    )
  }

  return (
    <div className="flex w-full flex-col gap-2 rounded-md">
      {(rules || []).map((rule, index) => {
        const isLast = index === (rules || []).length - 1
        return (
          <div
            key={index}
            className="text-text-foreground flex h-8 items-center"
          >
            {renderTreeLine(index, isLast)}
            <span className="sm:text-ilabel inline-flex h-full items-center pr-2 font-medium">
              If value is
            </span>
            <select
              value={rule.comparison}
              onChange={(e) => changeRule(index, 'comparison', e.target.value)}
              className="rounded-r-0 sm:text-ilabel border-main text-text-foreground focus:border-primary-600 focus:ring-3 focus:ring-primary-600/30 inline-flex h-full items-center rounded-l-md border border-r-0 bg-gray-50 py-1 pl-4 pr-7"
            >
              {Object.entries(operators).map(([op, display]) => {
                return (
                  <option key={op} value={op}>
                    is {display}
                  </option>
                )
              })}
            </select>
            <input
              type="text"
              name="value"
              id="value"
              className="w-30 rounded-l-0 sm:text-ilabel border-main hover:border-primary-600 focus:border-primary-600 focus:ring-3 focus:ring-primary-600/30 block h-full rounded-r-md border px-2 py-1"
              placeholder="0"
              value={rule.value}
              onChange={(e) => {
                changeRule(index, 'value', e.target.value)
              }}
            />
            <span className="sm:text-ilabel  inline-flex h-full items-center  rounded-none px-2 font-medium">
              , then show
            </span>
            <input
              type="text"
              name="text"
              id="text"
              className="sm:text-ilabel border-main hover:border-primary-600 focus:border-primary-600 focus:ring-3  focus:ring-primary-600/30 block h-full w-80 rounded-md px-2 py-1"
              placeholder="Display text (e.g. High, Low, Normal)"
              value={rule.text}
              onChange={(e) => {
                changeRule(index, 'text', e.target.value)
              }}
            />
            {/* TODO: implement color mapping in data-grid component */}
            {/* <span className="sm:text-ilabel  inline-flex h-full items-center rounded-none px-3 font-medium">
              also set color
            </span>
            <span className="focus-within:ring-primary-500 rounded-md border border-main px-0.5 focus-within:border-transparent focus-within:ring-2">
              <ColorSelect
                value={rule.colorTheme}
                onChange={(colorTheme) => {
                  changeRule(index, 'colorTheme', colorTheme?.value)
                }}
              />
            </span>
            <div className="flex-1"></div> */}
            <button
              type="button"
              className="text-text-foreground-disabled hover:text-primary-600 mx-2 cursor-pointer"
              aria-label="remove"
              onClick={() => removeRule(index)}
            >
              <LuTrash2 className="icon" aria-hidden="true" />
            </button>
          </div>
        )
      })}
      <Button
        type="button"
        role="secondary"
        className="w-fit flex-none py-1.5"
        aria-label="remove"
        onClick={addRule}
        icon={<LuPlus />}
      >
        Add Formatting Rule
      </Button>
    </div>
  )
}
