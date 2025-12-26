import { useFormContext, useWatch } from 'react-hook-form'
import { CopyButton } from '@sentio/ui-core'
import Web3 from 'web3'
import { memo } from 'react'

const web3 = new Web3()

export const CallDataCopy = memo(function CopyLabel() {
  const { control } = useFormContext()

  const functionInterface = useWatch({
    name: 'function',
    control
  })

  const functionParams = useWatch({
    name: 'functionParams',
    control
  })

  let text = ''
  try {
    text = web3.eth.abi.encodeFunctionCall(
      functionInterface,
      functionParams.map((item: { value: any }) => item.value)
    )
  } catch {
    text = ''
  }

  return (
    <CopyButton
      text={text}
      className="relative mr-4 inline-block w-fit"
      size={16}
    >
      <span className="hover:text-primary mr-1 inline-flex cursor-pointer">
        Copy Call Data
      </span>
    </CopyButton>
  )
})
