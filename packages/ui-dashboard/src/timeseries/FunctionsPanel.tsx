import { Tab } from '@headlessui/react'
import { Fragment, useEffect, useRef, useState } from 'react'
import { classNames } from '@sentio/ui-core'
import { BiCaretRight } from 'react-icons/bi'
import { FunctionDef, FunctionsCategories } from './functions'

interface Props {
  onClick: (func: FunctionDef) => void
  functionCategories?: typeof FunctionsCategories
  defaultFunc?: string
}

export function FunctionsPanel({
  onClick,
  functionCategories = FunctionsCategories,
  defaultFunc
}: Props) {
  const ulRef = useRef<HTMLUListElement>(null)
  const [selectedIdx, setSelectedIdx] = useState(0)
  useEffect(() => {
    if (!defaultFunc) return
    let targetIndex = 0
    Object.keys(functionCategories).forEach((category, idx) => {
      const func = functionCategories[category].find(
        (f) => f.name === defaultFunc
      )
      if (func) {
        targetIndex = idx
      }
    })
    setSelectedIdx(targetIndex)
    setTimeout(() => {
      const target = ulRef.current?.querySelector(
        `li[data-name="${defaultFunc}"]`
      )
      if (target) {
        target.scrollIntoView({ block: 'center' })
      }
    }, 0)
  }, [defaultFunc])
  return (
    <div className="bg-default-bg flex h-full overflow-hidden rounded-md">
      <Tab.Group vertical selectedIndex={selectedIdx} onChange={setSelectedIdx}>
        <Tab.List
          as="ul"
          className="native-scroller border-main flex w-44 shrink-0 flex-col flex-nowrap divide-y divide-gray-200 overflow-auto border-r"
        >
          {Object.keys(functionCategories).map((category, idx) => (
            <Tab as={Fragment} key={category}>
              {({ selected }) => (
                <li
                  onMouseOver={() => setSelectedIdx(idx)}
                  className={classNames(
                    selected
                      ? 'bg-primary-500 hover:bg-primary-600'
                      : 'bg-default-bg hover:bg-gray-50',
                    selected ? 'text-white' : 'text-foreground',
                    'flex cursor-pointer items-center justify-between p-2 text-sm font-medium'
                  )}
                >
                  <p
                    className={classNames(
                      'text-ilabel flex-1 truncate font-medium'
                    )}
                  >
                    {category}
                  </p>
                  <BiCaretRight
                    className={classNames('h-3 w-3 shrink-0 self-center')}
                  />
                </li>
              )}
            </Tab>
          ))}
        </Tab.List>
        <Tab.Panels className="flex-1">
          {Object.keys(functionCategories).map((category) => (
            <Tab.Panel
              as="ul"
              key={category}
              className="h-full divide-y overflow-y-auto"
              ref={ulRef}
            >
              {functionCategories[category]
                .filter((f) => !f.deprecated)
                .map((func) => (
                  <li
                    key={func.name}
                    className={classNames(
                      'group cursor-pointer space-y-1 px-2 py-1.5',
                      func.name === defaultFunc
                        ? 'bg-primary-600 dark:bg-primary-600 text-white'
                        : 'hover:bg-sentio-gray-100 dark:hover:bg-sentio-gray-400 text-text-foreground dark:hover:text-white'
                    )}
                    onClick={() => onClick(func)}
                    data-name={func.name}
                  >
                    <div className="flex items-center justify-between">
                      <p className="text-ilabel truncate font-medium">
                        {func.displayName || func.name}
                      </p>
                    </div>
                    <div className="flex">
                      <div
                        className={classNames(
                          'text-icontent flex items-center',
                          func.name === defaultFunc
                            ? 'text-white/80'
                            : 'text-text-foreground-secondary'
                        )}
                      >
                        <p>{func.description}</p>
                      </div>
                    </div>
                  </li>
                ))}
            </Tab.Panel>
          ))}
        </Tab.Panels>
      </Tab.Group>
    </div>
  )
}
