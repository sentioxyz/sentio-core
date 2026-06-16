import { Popover } from '@headlessui/react'
import { LuX as XIcon, LuChevronDown } from 'react-icons/lu'
import { useFloating, autoPlacement } from '@floating-ui/react'
import { produce } from 'immer'
import isEqual from 'lodash/isEqual'
import { HelpIcon, PopoverButton, classNames } from '@sentio/ui-core'
import { FunctionsPanel } from './FunctionsPanel'
import { ArgumentInput } from './ArgumentInput'
import { FunctionDef, FunctionMap } from './functions'
import type { ArgumentLike, FunctionLike, QueryLike } from '../types/metrics'

interface Props {
  value: QueryLike
  onChange: (value: QueryLike) => void
}

export function FunctionInput({ value, onChange }: Props) {
  const { x, y, refs, strategy } = useFloating({
    middleware: [autoPlacement()]
  })

  const onSelectFunc = (f: FunctionDef) => {
    onChange(
      produce(value, (draft) => {
        draft.functions = draft.functions || []
        draft.functions.push({
          name: f.name,
          arguments: f.defaultArguments || []
        })
      })
    )
  }

  const remove = (f: FunctionLike) => {
    const idx = (value.functions || []).indexOf(f)
    if (idx >= 0) {
      onChange(
        produce(value, (draft) => {
          draft.functions = draft.functions || []
          draft.functions.splice(idx, 1)
        })
      )
    }
  }

  function changeArgument(fidx: number, aidx: number, v: ArgumentLike) {
    onChange(
      produce(value, (draft) => {
        draft.functions = draft.functions || []
        const f = draft.functions[fidx]
        if (f) {
          f.arguments = f.arguments || []
          f.arguments[aidx] = v
        }
      })
    )
  }

  function changeFunction(fidx: number, f: FunctionDef) {
    onChange(
      produce(value, (draft) => {
        draft.functions = draft.functions || []
        const preFunc = draft.functions[fidx]
        let resetArg = true
        if (preFunc.arguments?.length === f.defaultArguments?.length) {
          const firstArg = preFunc.arguments?.[0]
          const firstDefaultArg = f.defaultArguments?.[0]
          if (firstArg && firstDefaultArg) {
            resetArg = isEqual(
              Object.keys(firstArg),
              Object.keys(firstDefaultArg)
            )
              ? false
              : true
          }
        }
        draft.functions[fidx] = {
          name: f.name,
          arguments: resetArg ? f.defaultArguments || [] : preFunc.arguments
        }
      })
    )
  }

  return (
    <>
      <Functions
        functions={value.functions || []}
        onRemove={remove}
        onChangeArgument={changeArgument}
        onChangeFunction={changeFunction}
      />
      <div className="inline-flex items-center">
        <div className="h-0.5 w-2.5 self-center bg-gray-300"></div>
        <Popover className="relative">
          {({ open }) => (
            <>
              <Popover.Button
                ref={refs.setReference}
                aria-label="Add function"
                className={classNames(
                  'text-ilabel focus:border-primary-600 focus:ring-primary-600/30 focus:ring-3 relative -ml-px inline-flex h-8 items-center space-x-2 rounded-md',
                  'border-main hover:border-primary-600 border bg-gray-100 px-4 font-normal',
                  open
                    ? 'text-text-foreground ring-1'
                    : 'text-text-foreground-disabled hover:text-text-foreground'
                )}
              >
                <span className="flex text-sm">f(x)</span>
                <HelpIcon text={'Add functions to query.'} />
              </Popover.Button>

              <Popover.Panel
                className="shadow-xs border-main z-10 mt-3 h-56 w-96 rounded-md border px-2 sm:px-0 lg:max-w-3xl"
                ref={refs.setFloating}
                style={{
                  position: strategy,
                  top: y ?? 0,
                  left: x ?? 0
                }}
              >
                {({ close }) => (
                  <FunctionsPanel
                    onClick={(f) => {
                      onSelectFunc(f)
                      close()
                    }}
                  />
                )}
              </Popover.Panel>
            </>
          )}
        </Popover>
      </div>
    </>
  )
}

function Functions({
  functions,
  onRemove,
  onChangeArgument,
  onChangeFunction
}: {
  functions: FunctionLike[]
  onRemove: (f: FunctionLike) => void
  onChangeArgument: (fIdx: number, argIdx: number, value: ArgumentLike) => void
  onChangeFunction?: (fIdx: number, f: FunctionDef) => void
}) {
  if (functions.length == 0) {
    return <></>
  }

  return (
    <>
      {functions.map((f, fi) => {
        const def = FunctionMap[f.name!]
        return (
          <div key={f.name} className="inline-flex items-center">
            <div className="h-0.5 w-2.5 self-center bg-gray-300"></div>
            <div
              className={classNames(
                'text-ilabel focus:outline-hidden text-text-foreground-secondary relative inline-flex items-center pl-2 font-normal',
                'border-main rounded-md border',
                'h-8'
              )}
            >
              <PopoverButton
                containerClassName="h-full border-r border-light pr-2 inline-flex items-center bg-gray-50"
                content={({ close }) => (
                  <div className="z-10 h-56 w-96 px-2 sm:px-0 lg:max-w-3xl">
                    <FunctionsPanel
                      onClick={(f) => {
                        onChangeFunction?.(fi, f)
                        close()
                      }}
                      defaultFunc={f.name}
                    />
                  </div>
                )}
              >
                <span className="hover:text-primary-600 text-text-foreground inline-flex cursor-pointer flex-nowrap items-center gap-1">
                  {def.displayName || f.name}
                  <LuChevronDown className="h-4 w-4" />
                </span>
              </PopoverButton>
              {def.arguments.map((arg, i) => (
                <ArgumentInput
                  className="sm:text-ilabel hover:border-primary-600 focus:ring-3 focus:ring-primary-600/30 block w-full border border-transparent pl-4"
                  key={'arg_' + i}
                  argument={arg}
                  value={f.arguments && f.arguments[i]}
                  onChange={(v) => onChangeArgument(fi, i, v)}
                />
              ))}
              <button
                type={'button'}
                className={
                  'text-text-foreground-disabled hover:text-foreground h-full rounded-r-md px-2 hover:bg-gray-100'
                }
                aria-label="remove function"
                onClick={() => onRemove(f)}
              >
                <XIcon className="h-4.5 w-4.5" aria-hidden="true" />
              </button>
            </div>
          </div>
        )
      })}
    </>
  )
}
