import { StateVariable } from '@sentio/debugger-common'
import { ContractAddress, RawParam, CopyableParam } from '../ContractComponents'

interface Props {
  data: StateVariable
}

function Param({ data }: { data: any }) {
  const type = typeof data
  if (['string', 'number', 'boolean'].includes(type)) {
    return <CopyableParam value={String(data)} />
  }
  return <RawParam data={data} />
}

export function DecodedVariable(props: Props) {
  const { data } = props
  const { type, decoded } = data

  if (!type) {
    return null
  }

  const renderValue = (keys: string[], value: any, depth = 0): JSX.Element => {
    if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
      return (
        <span className="inline-flex flex-wrap gap-2">
          {Object.entries(value).map(([nestedKey, nestedValue]) => (
            <span key={nestedKey}>
              {renderValue([...keys, nestedKey], nestedValue, depth + 1)}
            </span>
          ))}
        </span>
      )
    }

    const parseTypeStructure = (typeStr: string) => {
      const parts: string[] = []
      let current = typeStr

      while (current.includes('mapping')) {
        const match = current.match(/mapping\s*\(\s*(\w+)\s*=>\s*(.+)\)/)
        if (match) {
          parts.push(match[1])
          current = match[2].trim()
        } else {
          break
        }
      }
      parts.push(current)
      return parts
    }

    const typeStructure = parseTypeStructure(type.type || '')

    return (
      <span className="inline-flex items-center gap-1">
        <span className="text-magenta/70 dark:text-magenta-800 font-medium">
          {type.name}
        </span>
        {keys.map((key, index) => {
          const keyType = typeStructure[index]?.toLowerCase()
          const isAddress = keyType === 'address'

          return (
            <span key={index} className="inline-flex items-center gap-1">
              <span className="text-gray-500">[</span>
              {isAddress ? (
                <ContractAddress address={key} />
              ) : (
                <Param data={key} />
              )}
              <span className="text-gray-500">]</span>
            </span>
          )
        })}
        <span className="text-gray-500">=</span>
        <Param data={value} />
      </span>
    )
  }

  const renderSimpleValue = (value: string) => {
    const isPrimitiveType =
      type.type &&
      ['uint', 'int', 'bool', 'string', 'bytes'].some(
        (primitiveType) =>
          type.type!.startsWith(primitiveType) || type.type === primitiveType
      )

    return (
      <span className="inline-flex items-center gap-1">
        <span className="text-magenta/70 dark:text-magenta-800 font-medium">
          {type.name}
        </span>
        <span className="text-gray-500">=</span>
        {isPrimitiveType ? (
          <CopyableParam value={value} />
        ) : (
          <Param data={value} />
        )}
      </span>
    )
  }

  const renderDecodedData = () => {
    if (decoded === null || decoded === undefined) {
      return null
    }

    // Handle primitive values (string, number, boolean)
    if (typeof decoded !== 'object') {
      return renderSimpleValue(String(decoded))
    }

    if (type.type && type.type.includes('mapping')) {
      return (
        <span className="inline-flex flex-wrap gap-2">
          {Object.entries(decoded).map(([key, value]) => (
            <span key={key}>{renderValue([key], value)}</span>
          ))}
        </span>
      )
    }

    const entries = Object.entries(decoded)
    if (entries.length === 1) {
      const [, value] = entries[0]
      return renderSimpleValue(String(value))
    }

    return (
      <span className="inline-flex flex-wrap gap-2">
        {entries.map(([key, value]) => (
          <span key={key}>{renderValue([key], value)}</span>
        ))}
      </span>
    )
  }

  return <div className="inline-block">{renderDecodedData()}</div>
}
