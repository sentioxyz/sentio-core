import { ParamType } from './types'
import { HexNumber } from './HexNumber'
import { useContext, memo, ReactElement } from 'react'
import {
  isUndefined,
  isNull,
  isArray,
  isString,
  isObject,
  map,
  isNumber
} from 'lodash'
import isNumeric from 'validator/lib/isNumeric'
import { PopoverTooltip } from '../common/DivTooltip'
import { useAddressTag } from '../utils/use-tag'
import { CopyButton } from '../common/CopyButton'
import { cx } from 'class-variance-authority'
import EtherLink from './EtherLink'
import {
  isArrayType,
  isAddressType,
  toChecksumAddress,
  safeNativize
} from './helpers'
import {
  SenderContext,
  ReceiverContext,
  ChainIdContext,
  GlobalQueryContext
} from './transaction-context'
import { useFallbackName } from './use-fallback-name'
import { IoMdInformationCircleOutline } from 'react-icons/io'
import { OpenContractContext } from '../utils/extension-context'
import { useDarkMode } from '../utils/extension-context'
import { useMobile } from '../utils/use-mobile'
import JsonView from '@uiw/react-json-view'
import { lightTheme } from '@uiw/react-json-view/light'
import { darkTheme } from '@uiw/react-json-view/dark'

const numberFormat = new Intl.NumberFormat('en-US')
export function displayNativeValue(value?: any): React.ReactNode {
  if (isUndefined(value) || isNull(value)) {
    return
  }

  if (!value.kind) {
    if (isArray(value)) {
      return value.map((v) => displayNativeValue(v)).join(', ')
    }
    if (isString(value) && isNumeric(value)) {
      return numberFormat.format(value as any)
    }
    if (isObject(value)) {
      return `{${map(value, (v, k) => `${k} = ${displayNativeValue(v)}`).join(', ')}}`
    }
    return value.toString()
  }

  if (value.kind === 'error' && (value.error as any)?.raw) {
    return (value.error as any).raw
  }

  const native = safeNativize(value)
  if (native !== undefined) {
    if (isNumber(native)) {
      return (native as number).toLocaleString()
    } else if (isString(native) && native.startsWith('0x')) {
      return (
        <HexNumber
          data={native as string}
          className="-mt-1 ml-0.5 translate-y-1 text-xs"
          avatar
        />
      )
    }
    return native.toLocaleString()
  }
  return
}

export function displayNativeValueInLines(value?: any): React.ReactNode {
  if (isUndefined(value) || isNull(value)) {
    return
  }

  if (!value.kind) {
    if (isArray(value)) {
      return (
        <div className="inline-block space-y-1">
          {value.map((v) => (
            <div key={JSON.stringify(v)}>{displayNativeValueInLines(v)}</div>
          ))}
        </div>
      )
    }
    if (isString(value) && isNumeric(value)) {
      return numberFormat.format(value as any)
    }
    if (isObject(value)) {
      const { name, type, value: v } = value as any
      if (name && type && v) {
        return <ContractParam data={value as any} />
      }

      return (
        <div>
          {map(value, (v, k) => `${k} = ${displayNativeValueInLines(v)}`).join(
            ', '
          )}
        </div>
      )
    }
    return value.toString()
  }

  if (value.kind === 'error' && (value.error as any)?.raw) {
    return (value.error as any).raw
  }

  const native = safeNativize(value)
  if (native !== undefined) {
    if (isNumber(native)) {
      return (native as number).toString()
    } else if (isString(native) && native.startsWith('0x')) {
      return (
        <HexNumber
          data={native as string}
          className="-mt-1 ml-0.5 translate-y-1 text-xs"
          avatar
        />
      )
    }
    return native.toString()
  }
  return
}

function noSelect(e: React.MouseEvent) {
  e.stopPropagation()
}

function isEqualAddress(a?: string, b?: string) {
  return a?.toLowerCase() === b?.toLowerCase()
}

export const ContractAddressComponent = ({
  address: _address,
  tooltipClassName = '!inline-flex',
  containerClassName,
  toLowerCase = true,
  showAvatar,
  name: _name,
  noPrefix,
  isFallbackName,
  tooltipWidth = 'max-w-[500px]',
  toChecksum = true,
  linkParam,
  type = 'Contract'
}: {
  address: string
  tooltipClassName?: string
  containerClassName?: string
  toLowerCase?: boolean
  showAvatar?: boolean
  name?: string
  noPrefix?: boolean
  isFallbackName?: boolean
  tooltipWidth?: string
  toChecksum?: boolean
  linkParam?: string
  type?: string
}) => {
  const sender = useContext(SenderContext) || 'sender'
  const receiver = useContext(ReceiverContext) || 'receiver'
  const chainId = useContext(ChainIdContext)
  const address = toLowerCase ? _address?.toLowerCase() : _address
  const checksumAddress = !toChecksum ? address : toChecksumAddress(address)
  const isSenderAddress = isEqualAddress(address, sender)
  const isReceiverAddress = isEqualAddress(address, receiver)
  const globalQuery = useContext(GlobalQueryContext)
  const openContractAddress = useContext(OpenContractContext)
  const name =
    '0x0000000000000000000000000000000000000001' === _address
      ? 'Precompiled'
      : _name
  const isMobile = useMobile()

  return (
    <section className={cx('inline-block', containerClassName)}>
      {!noPrefix && isSenderAddress ? (
        <span className="mr-0.5 text-orange-700">[Sender]</span>
      ) : null}
      {!noPrefix && isReceiverAddress ? (
        <span className="mr-0.5 text-orange-700">[Receiver]</span>
      ) : null}
      <PopoverTooltip
        usePortal
        offsetOptions={2}
        className={tooltipClassName}
        strategy="fixed"
        hideArrow
        maxWidth={tooltipWidth}
        text={
          <div
            className="text-ilabel text-gray space-y-2 overflow-hidden px-2 py-1"
            onClick={noSelect}
          >
            <div className="flex w-full items-center justify-between gap-4">
              {name ? (
                <div className="flex items-center gap-2">
                  <div>
                    {name}
                    <span className="text-orange ml-1 text-xs">
                      {isFallbackName ? '(from contract source code)' : ''}
                    </span>
                  </div>
                  <div className="relative">
                    <CopyButton text={name} />
                  </div>
                </div>
              ) : isSenderAddress ? null : (
                <div className="text-sm text-gray-400">{type}</div>
              )}
              {isSenderAddress ? null : (
                <div
                  className="bg-primary-300 hover:bg-primary-400 cursor-pointer rounded px-1.5 py-0.5 font-[system-ui] text-white"
                  onClick={() => {
                    if (openContractAddress && chainId) {
                      return openContractAddress(address, chainId)
                    }
                    const { owner, slug } = globalQuery
                    if (owner && slug) {
                      window.open(
                        `/${owner}/${slug}/contract/${chainId}/${address}${linkParam || ''}`
                      )
                    } else {
                      window.open(
                        `/contract/${chainId}/${address}${linkParam || ''}`
                      )
                    }
                  }}
                >
                  view
                </div>
              )}
            </div>
            <div className="flex items-center gap-2">
              <div className="whitespace-normal break-all font-mono">
                {checksumAddress}
              </div>
              <div className="relative">
                <CopyButton text={checksumAddress} />
              </div>
              <EtherLink
                address={checksumAddress}
                chainId={chainId}
                trigger="static"
              />
            </div>
          </div>
        }
      >
        <div className="flex items-center">
          <span
            className={cx(
              'text-primary-800/80 flex-1 cursor-pointer truncate whitespace-nowrap',
              name ? '' : 'font-mono'
            )}
          >
            {name ||
              (isMobile
                ? checksumAddress?.substring(0, 6) +
                  '...' +
                  checksumAddress?.substring(checksumAddress.length - 4)
                : checksumAddress)}
          </span>
          {isFallbackName ? (
            <IoMdInformationCircleOutline className="flex-0 ml-1 inline-block h-4 w-4 align-text-bottom text-gray-400" />
          ) : null}
        </div>
      </PopoverTooltip>
    </section>
  )
}

export const ContractAddressWithName = (props: {
  address: string
  tooltipClassName?: string
  containerClassName?: string
  name: string
}) => {
  return <ContractAddressComponent isFallbackName={false} {...props} />
}

export const ContractAddress = (props: {
  address: string
  tooltipClassName?: string
  containerClassName?: string
  toLowerCase?: boolean
  showAvatar?: boolean
  fallbackName?: string // tag service fallback name
  noPrefix?: boolean
  linkParam?: string // contract address link parameters
}) => {
  const { address, fallbackName } = props
  const chainId = useContext(ChainIdContext)
  const { data } = useAddressTag(address)
  const defaultName = useFallbackName(address, fallbackName)
  const isFallbackName = !!defaultName && (!data || !data?.primaryName)

  return (
    <ContractAddressComponent
      name={data?.primaryName || defaultName}
      isFallbackName={isFallbackName}
      {...props}
    />
  )
}

// export const ContractAddressWithFetch = (props: {
//   address: string
//   tooltipClassName?: string
//   containerClassName?: string
//   toLowerCase?: boolean
//   showAvatar?: boolean
//   noPrefix?: boolean
// }) => {
//   const { address } = props
//   const chainId = useContext(ChainIdContext)
//   const { data } = useAddressTag(address)
//   const { data: serverData } = useAddressTagFromServer(address, chainId)
//   const dataToUse = data || serverData
//   const defaultName = useFallbackName(address)
//   const isFallbackName = !!defaultName && (!data || !data?.primaryName)

//   return (
//     <ContractAddressComponent name={dataToUse?.primaryName || defaultName} isFallbackName={isFallbackName} {...props} />
//   )
// }

const stopPropagation = (e: React.MouseEvent) => {
  e.stopPropagation()
}

export const RawParam = memo(function RawParam({
  data,
  showAll
}: {
  data?: any
  showAll?: boolean
}) {
  const strValue = isObject(data) ? JSON.stringify(data) : data?.toString()
  if (!strValue) {
    return null
  }
  if (strValue?.length <= 10 || showAll) {
    return <CopyableParam value={strValue} />
  }
  return (
    <span className="inline-block">
      <PopoverTooltip
        strategy="fixed"
        hideArrow
        maxWidth="max-w-[500px]"
        text={
          <div
            className="min-w-[200px] max-w-[500px]"
            onClick={stopPropagation}
          >
            <div>{strValue?.substring(0, 10)}</div>
            <div className="whitespace-normal break-all">
              {strValue?.substring(10)}
              <span className="ml-2 inline-block">
                <CopyButton text={strValue} />
              </span>
            </div>
          </div>
        }
      >
        <span className="text-gray hover:bg-primary-50 cursor-pointer rounded border px-1 py-0.5 font-medium">
          raw data
        </span>
      </PopoverTooltip>
    </span>
  )
})

export const CopyableParam = memo(function CopyableParam({
  rawValue,
  value,
  tooltipClassName = '!inline-flex'
}: {
  rawValue?: string
  value: string | React.ReactNode
  tooltipClassName?: string
}) {
  const isDarkMode = useDarkMode()
  const NodeElement = <span className="text-primary-800/60">{value}</span>

  if (isString(value)) {
    let strValue = ''
    let jsonData: any = null

    if (rawValue !== undefined) {
      strValue = isObject(rawValue)
        ? JSON.stringify(rawValue)
        : rawValue.toString()
    } else {
      strValue = value
    }

    // Try to parse as JSON for react-json-view
    try {
      jsonData = JSON.parse(strValue)
    } catch {
      jsonData = null
    }

    const isValidJson = jsonData !== null && typeof jsonData === 'object'

    return (
      <PopoverTooltip
        offsetOptions={2}
        className={tooltipClassName}
        strategy="fixed"
        placementOption="bottom-start"
        hideArrow
        maxWidth="max-w-[600px]"
        text={
          <div className="min-w-0" onClick={stopPropagation}>
            <div className="sticky top-0 z-10 mb-2 flex items-center justify-end border-b border-gray-200 pb-2">
              <CopyButton text={strValue} />
            </div>
            <div className="scrollbar-thin max-h-[400px] overflow-auto">
              {isValidJson ? (
                <div className="min-w-[300px]">
                  <JsonView
                    value={jsonData}
                    style={isDarkMode ? darkTheme : lightTheme}
                  />
                </div>
              ) : (
                <pre className="text-gray whitespace-pre-wrap break-all font-mono text-xs leading-relaxed">
                  {strValue}
                </pre>
              )}
            </div>
          </div>
        }
      >
        {NodeElement}
      </PopoverTooltip>
    )
  }
  return value
})

export interface ContractParamProps {
  data: ParamType
  showRaw?: boolean
  isLast?: boolean
}

const isTypedValue = (data: any) => {
  return (
    data?.name !== undefined &&
    data?.type !== undefined &&
    data?.value !== undefined
  )
}

export const ContractParam = memo(function ContractParam({
  data,
  isLast,
  showRaw
}: ContractParamProps) {
  const { type, value, name } = data

  let valueNode: ReactElement

  if (isArrayType(type) && isArray(value)) {
    valueNode = (
      <>
        <span className="text-gray mr-1">[</span>
        {(value as any[])?.map((v, index) => {
          if (isTypedValue(v)) {
            return (
              <ContractParam
                key={`${JSON.stringify(v)}_${index}`}
                data={v}
                isLast={index === value.length - 1}
                showRaw={showRaw}
              />
            )
          }
          const subType = isArray(v) ? type : type.replace('[]', '')
          const key = isObject(v) ? JSON.stringify(v) : `${v}_${index}`
          return (
            <ContractParam
              key={key}
              data={{ type: subType, value: v }}
              isLast={index === value.length - 1}
              showRaw={showRaw}
            />
          )
        })}
        <span className="text-gray ml-1">]</span>
      </>
    )
  } else if (isAddressType(type)) {
    valueNode = <ContractAddress address={value as string} />
  } else if (type === 'bytes' && isString(value) && value?.length > 10) {
    valueNode = <RawParam data={value as string} showAll={showRaw} />
  } else {
    const _value = displayNativeValue(value as string)
    valueNode = (
      <CopyableParam
        value={_value}
        rawValue={isObject(value) ? JSON.stringify(value) : value?.toString()}
      />
    )
  }

  return (
    <section className="inline-flex">
      {name && (
        <>
          <span className="flex-0 text-gray">{name}</span>
          <span className="text-gray mx-1">=</span>
        </>
      )}
      <span className="flex-1 basis-0">{valueNode}</span>
      {isLast ? null : <span className="mx-1">,</span>}
    </section>
  )
})
