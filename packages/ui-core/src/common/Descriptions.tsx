import React, { ReactNode } from 'react'
import { classNames } from '../utils/classnames'
import {
  isObjectLike,
  map,
  isFunction,
  isEmpty,
  isString,
  isNumber
} from 'lodash'

export type DataType = {
  key?: React.Key
  label?: ReactNode
  value?: any
  span?: number
}

interface Props {
  data: DataType[]
  labelClassName?: string
  labelStyle?: React.CSSProperties
  valueClassName?: string
  valueStyle?: React.CSSProperties
  className?: string
  trClassName?: string
  colon?: ReactNode
  renderLabel?: (item: DataType) => ReactNode
  renderValue?: (item: DataType) => ReactNode
}

function safeToString(value: any) {
  if (isString(value) || isNumber(value)) {
    return value
  }
  try {
    return JSON.stringify(value)
  } catch {
    return ''
  }
}

export const Descriptions = (props: Props) => {
  const {
    data,
    labelStyle,
    valueStyle,
    className,
    labelClassName,
    valueClassName,
    trClassName,
    colon,
    renderLabel,
    renderValue
  } = props
  return (
    <table className={classNames('w-full border-collapse', className)}>
      <tbody>
        {data.map((item, index) => {
          return (
            <tr key={item.key ?? index} className={trClassName}>
              <td
                className={classNames(
                  'text-gray text-ilabel font-ilabel w-px whitespace-nowrap pr-8 align-text-bottom',
                  labelClassName
                )}
                style={labelStyle}
              >
                {isFunction(renderLabel) ? renderLabel?.(item) : item.label}
              </td>
              {colon}
              <td
                className={classNames(
                  'text-ilabel font-ilabel',
                  valueClassName
                )}
                style={valueStyle}
              >
                {React.isValidElement(item.value) ? (
                  item.value
                ) : isObjectLike(item.value) ? (
                  isEmpty(item.value) ? (
                    <div className="text-gray-400">{'{ }'}</div>
                  ) : (
                    <div className="space-y-2">
                      <div className="text-gray-400">{'{'}</div>
                      <Descriptions
                        {...props}
                        data={map(item.value, (value, label) => ({
                          key: `${item.key}.${label}`,
                          label,
                          value
                        }))}
                      />
                      <div className="text-gray-400">{'}'}</div>
                    </div>
                  )
                ) : isFunction(renderValue) ? (
                  renderValue?.(item)
                ) : (
                  safeToString(item.value)
                )}
              </td>
            </tr>
          )
        })}
      </tbody>
    </table>
  )
}
