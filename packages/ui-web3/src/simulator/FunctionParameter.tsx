import { useState, useEffect } from 'react'
import {
  useController,
  useFormContext,
  useWatch,
  Control
} from 'react-hook-form'
import { DebounceInput } from 'react-debounce-input'
import Web3 from 'web3'
import { classNames, CopyButton } from '@sentio/ui-core'
import {
  ExclamationTriangleIcon,
  CheckCircleIcon
} from '@heroicons/react/24/solid'
import type { AbiFunction } from './types'
import { CallDataCopy } from './CallDataCopy'

interface Props {
  control: Control<any>
  className?: string
  lineClassName?: string
  labelClassName?: string
  inputClassName?: string
  copyBtnClassName?: string
  copyAllClassName?: string
}

const web3 = new Web3()

const paramsToDefaultValue = (params: AbiFunction['inputs']) => {
  return params.map((param) => ({
    name: param.name,
    value: ''
  }))
}

interface ObjectTextAreaProps<T> {
  value: T
  onChange?: (v: T) => void
  placeholder?: string
}

function ObjectTextArea<T>({
  value,
  onChange,
  placeholder
}: ObjectTextAreaProps<T>) {
  const [text, setText] = useState<string>('')
  const [error, setError] = useState(false)

  useEffect(() => {
    setText((pre) => {
      try {
        const preObject = JSON.parse(pre)
        if (JSON.stringify(preObject) === JSON.stringify(value)) {
          return pre
        }
      } catch {
        // ignore
      }
      return value ? JSON.stringify(value) : ''
    })
  }, [value])

  useEffect(() => {
    if (!text) {
      return
    }
    try {
      const parsedValue = JSON.parse(text)
      setError(false)
      if (JSON.stringify(parsedValue) === JSON.stringify(value)) {
        return
      }
      onChange?.(parsedValue)
    } catch (e) {
      setError(true)
    }
  }, [text, value, onChange])

  return (
    <DebounceInput
      element="textarea"
      value={text}
      onChange={(e) => setText(e.target.value)}
      debounceTimeout={300}
      forceNotifyOnBlur
      className={`w-full rounded border p-2 font-mono text-xs font-normal ${
        error
          ? 'border-red-500 focus:border-red-500 focus:ring-red-500'
          : 'border-gray-300'
      }`}
      placeholder={placeholder}
    />
  )
}

export const FunctionParameter = ({
  control,
  className,
  lineClassName,
  labelClassName,
  inputClassName,
  copyBtnClassName,
  copyAllClassName
}: Props) => {
  const { getValues } = useFormContext()
  const [error, setError] = useState('')

  const params = useWatch({
    name: 'function.inputs',
    control
  })

  const { field } = useController({
    name: 'functionParams',
    control,
    defaultValue: paramsToDefaultValue(params || [])
  })

  const onChange = (index: number, value: string | boolean) => {
    const params = field.value
    params[index].value = value
    field.onChange(params)

    setTimeout(() => {
      const functionInterface = getValues('function')
      try {
        web3.eth.abi.encodeFunctionCall(
          functionInterface,
          params?.map((item: any) => item.value)
        )
        setError('')
      } catch (e: any) {
        setError(e?.toString?.() ?? 'Error happens when encode function call')
      }
    }, 0)
  }

  if (!params || params.length === 0) {
    return null
  }

  return (
    <div className={`mt-2 rounded-md py-2 ${error ? 'bg-red-50' : ''}`}>
      <div className="flex w-full items-center gap-2">
        <div className="text-sm font-bold text-gray-900">Input Parameters</div>
        {error ? (
          <div className="flex items-center gap-1 text-red-600" title={error}>
            <ExclamationTriangleIcon className="h-4 w-4" />
          </div>
        ) : (
          <CheckCircleIcon
            className="h-4 w-4 text-green-600"
            title="Successfully decoded function parameters"
          />
        )}
      </div>
      <div
        className={`relative mt-4 space-y-4 text-xs font-medium ${className || ''}`}
      >
        {params.map((param: any, index: number) => {
          const isComplexType =
            param.type && (param.type.includes('[]') || param.type === 'tuple')
          const textValue = isComplexType
            ? field.value[index]?.value
              ? JSON.stringify(field.value[index]?.value)
              : ''
            : field.value[index]?.value

          return (
            <div
              key={`${param.name}.${index}`}
              className={`space-y-2 ${lineClassName || ''}`}
            >
              <div className={`w-full text-sm ${labelClassName || ''}`}>
                <span className="font-medium text-gray-900">{param.name}</span>
                <span className="ml-2 text-xs font-normal text-gray-500">
                  {param.type}
                </span>
              </div>
              <div
                className={`text-xs font-medium text-gray-900 ${inputClassName || ''}`}
              >
                {isComplexType ? (
                  <ObjectTextArea
                    value={field.value[index]?.value}
                    onChange={(val) => onChange(index, val)}
                    placeholder={param.internalType}
                  />
                ) : param.type === 'bool' ? (
                  <span className="inline-flex items-center">
                    <input
                      type="checkbox"
                      className="mr-1 rounded border-gray-300"
                      checked={field.value[index]?.value}
                      onChange={(e) => onChange(index, e.target.checked)}
                    />
                    <span className="ml-1">
                      {field.value[index]?.value ? 'True' : 'False'}
                    </span>
                  </span>
                ) : (
                  <input
                    value={field.value[index]?.value}
                    onChange={(e) => onChange(index, e.target.value)}
                    className="w-full rounded border border-gray-300 p-2 font-mono font-normal"
                    placeholder={param.internalType}
                  />
                )}
              </div>
              <div className={copyBtnClassName}>
                <CopyButton text={textValue} />
              </div>
            </div>
          )
        })}
        <div className={classNames('relative px-4', copyAllClassName)}>
          <CallDataCopy />
        </div>
      </div>
    </div>
  )
}
