import { RadioSelect } from '@sentio/ui-core'
import { Suspense } from 'react'
import { useFormContext } from 'react-hook-form'
import Web3 from 'web3'
import { useSimulatorContext } from './SimulatorContext'

export enum InputType {
  RawData = 'rawdata',
  ABI = 'abi'
}

const inputOptions = [
  {
    name: 'Enter raw input data',
    value: true
  },
  {
    name: 'Choose function and parameters',
    value: false
  }
]

interface Props {
  inputType: boolean
  onChange: (t: boolean) => void
}

const _CallDataSwitch = ({ inputType, onChange }: Props) => {
  const { getValues, setValue } = useFormContext()
  const { contractFunctions } = useSimulatorContext()
  const { wfunctions } = contractFunctions

  return (
    <RadioSelect
      options={inputOptions}
      value={inputType}
      onChange={(key) => {
        onChange(key)
        // decode function params from call data
        if (!key) {
          const input = getValues('input')
          if (input) {
            try {
              const web3 = new Web3()
              const funSigText = input.slice(0, 10)
              const paramsText = input.slice(10)
              const targetFunction = wfunctions?.find((item) => {
                return web3.eth.abi.encodeFunctionSignature(item) === funSigText
              })
              const decoded = web3.eth.abi.decodeParameters(
                targetFunction?.inputs || [],
                paramsText
              )
              const params = targetFunction?.inputs.map(({ name }: any) => {
                return {
                  name,
                  value: decoded[name]
                }
              })
              setValue('function', targetFunction)
              setValue('functionParams', params, { shouldTouch: true })
            } catch {
              // do nothing
            }
          }
        } else {
          // encode call data
          const functionInterface = getValues('function')
          const functionParams = getValues('functionParams')
          let encodedInput = ''
          try {
            const web3 = new Web3()
            encodedInput = web3.eth.abi.encodeFunctionCall(
              functionInterface,
              functionParams?.map((item: any) => item.value)
            )
            setValue('input', encodedInput, { shouldTouch: true })
          } catch (e) {
            encodedInput = ''
          }
        }
      }}
    />
  )
}

export const CallDataSwitch = (props: Props) => {
  return (
    <Suspense>
      <_CallDataSwitch {...props} />
    </Suspense>
  )
}
