import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { ContractItemType } from '@/lib/use-global-store'
import { useCallback, useRef, useState } from 'react'

interface Props {
  data: ContractItemType[]
  onSelect?: (item: ContractItemType) => void
}

export const ContractSelect = ({ data, onSelect }: Props) => {
  const [value, setValue] = useState('')
  const dataRef = useRef(data)
  dataRef.current = data
  const onValueChange = useCallback(
    (value: string) => {
      setValue(value)
      const item = dataRef.current.find((i) => i.id === value)
      if (item && onSelect) {
        onSelect(item)
      }
    },
    [onSelect]
  )
  return (
    <Select value={value} onValueChange={onValueChange}>
      <SelectTrigger>
        <SelectValue>Select a contract</SelectValue>
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          <SelectLabel>Contract</SelectLabel>
          {data.map((item) => {
            return (
              <SelectItem key={item.id} value={item.id}>
                {item.name}
              </SelectItem>
            )
          })}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}
