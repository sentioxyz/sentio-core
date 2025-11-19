import { useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useLocalStorage } from '@/lib/use-localstorage'

export const SettingContent = ({
  label,
  placeholder,
  storageId
}: {
  label: string
  placeholder: string
  storageId: string
}) => {
  const ref = useRef<HTMLInputElement>(null)
  const [value, setValue] = useLocalStorage(storageId, '')
  const [enableSave, setEnableSave] = useState(false)
  useEffect(() => {
    ref.current!.value = value
  }, [value])
  return (
    <div className="flex w-full items-center space-x-2">
      <label className="w-20 shrink-0 whitespace-nowrap pr-2 pt-2 text-right text-xs font-medium">{label}:</label>
      <Input
        type="text"
        placeholder={placeholder}
        ref={ref}
        className="h-8 text-xs"
        onChange={() => {
          setEnableSave(ref.current?.value !== value)
        }}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            setValue(ref.current?.value || '')
            setEnableSave(false)
          }
        }}
      />
      <Button
        disabled={!enableSave}
        type="submit"
        onClick={() => {
          setValue(ref.current?.value || '')
          setEnableSave(false)
        }}
        variant="outline"
        size="sm"
        className="px-2"
      >
        {enableSave ? (
          <i className="fa-regular fa-floppy-disk text-primary text-base"></i>
        ) : (
          <i className="fa-regular fa-circle-check text-success text-base"></i>
        )}
      </Button>
    </div>
  )
}
