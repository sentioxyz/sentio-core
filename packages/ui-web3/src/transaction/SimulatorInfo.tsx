import { Collapse, BD } from '@sentio/ui-core'
import React, { ReactNode } from 'react'
import { ArrowRightCircleIcon } from '@heroicons/react/24/outline'
import { ContractAddress } from './ContractComponents'
import { cx as classNames } from 'class-variance-authority'

export interface BlockOverride {
  number?: string
  timestamp?: string
  baseFeePerGas?: string
  gasLimit?: string
  coinbase?: string
  difficulty?: string
  prevRandao?: string
  blockHash?: Record<string, string>
  [key: string]: any
}

export interface StateOverride {
  balance?: string
  nonce?: string
  code?: string
  state?: Record<string, string>
  stateDiff?: Record<string, string>
}

export interface SimulationData {
  blockOverride?: BlockOverride
  sourceOverrides?: Record<string, string>
  stateOverrides?: Record<string, StateOverride>
}

interface Props {
  simulationData?: SimulationData
  className?: string
  // Optional: custom renderer for compilation tags
  renderCompilationTag?: (compilationId: string) => ReactNode
}

export const SimulatorInfo: React.FC<Props> = ({
  simulationData,
  className,
  renderCompilationTag
}) => {
  if (
    (!simulationData?.sourceOverrides ||
      Object.keys(simulationData.sourceOverrides).length === 0) &&
    (!simulationData?.stateOverrides ||
      Object.keys(simulationData.stateOverrides).length === 0) &&
    (!simulationData?.blockOverride ||
      Object.keys(simulationData.blockOverride).length === 0)
  ) {
    return null
  }

  return (
    <Collapse
      title="Simulation Overrides"
      className={classNames('hover:relative hover:z-[1]', className)}
    >
      <div className="overflow rounded-md border">
        {simulationData?.blockOverride &&
        Object.keys(simulationData.blockOverride).length > 0 ? (
          <div className="space-y-2 pb-2">
            <div className="text-ilabel text-gray rounded-t-md bg-gray-50 px-2 py-1 font-medium dark:bg-gray-800">
              Block Overrides
            </div>
            <div className="space-y-2 px-2">
              {Object.entries(simulationData.blockOverride)
                .filter(([key]) => key !== 'blockHash')
                .map(([key, value]) => {
                  let displayValue: string = String(value)
                  if (typeof value === 'string' && value.startsWith('0x')) {
                    try {
                      const hex = value.slice(2) || '0'
                      const bigIntValue = BigInt(`0x${hex}`)
                      displayValue = bigIntValue.toString()
                      if (key === 'timestamp') {
                        const timestampNum = Number(bigIntValue)
                        displayValue = new Date(
                          timestampNum * 1000
                        ).toUTCString()
                      }
                    } catch (error) {
                      console.error(`Error parsing ${key}: ${value}`, error)
                      displayValue = String(value) // fallback to original
                    }
                  }
                  return (
                    <div key={key} className="flex items-center space-x-2">
                      <span className="text-ilabel text-gray capitalize">
                        {key}
                      </span>
                      <ArrowRightCircleIcon className="h-4 w-4 text-gray-400" />
                      <span className="text-ilabel text-gray">
                        {displayValue}
                      </span>
                    </div>
                  )
                })}
              {simulationData.blockOverride.blockHash &&
              Object.keys(simulationData.blockOverride.blockHash).length > 0 ? (
                <div className="space-y-1">
                  <div className="text-ilabel text-gray font-medium">
                    Block Hashes
                  </div>
                  {Object.entries(simulationData.blockOverride.blockHash).map(
                    ([offset, hash]) => (
                      <div key={offset} className="flex items-center space-x-2">
                        <span className="text-ilabel text-gray">
                          Offset {offset}
                        </span>
                        <ArrowRightCircleIcon className="h-4 w-4 text-gray-400" />
                        <span className="text-ilabel text-gray font-mono">
                          {hash}
                        </span>
                      </div>
                    )
                  )}
                </div>
              ) : null}
            </div>
          </div>
        ) : null}

        {simulationData?.sourceOverrides &&
        Object.keys(simulationData.sourceOverrides).length > 0 ? (
          <div className="space-y-2 pb-2">
            <div className="text-ilabel text-gray rounded-t-md bg-gray-50 px-2 py-1 font-medium dark:bg-gray-800">
              Source Overrides
            </div>
            <div className="space-y-2 px-2">
              {Object.entries(simulationData.sourceOverrides).map(
                ([address, compilationId]) => (
                  <div key={address} className="flex items-center space-x-2">
                    <ContractAddress address={address} />
                    <ArrowRightCircleIcon className="h-4 w-4 text-gray-400" />
                    <span className="text-ilabel text-gray">
                      {renderCompilationTag ? (
                        renderCompilationTag(compilationId)
                      ) : (
                        <span className="font-mono text-sm">
                          {compilationId}
                        </span>
                      )}
                    </span>
                  </div>
                )
              )}
            </div>
          </div>
        ) : null}

        {simulationData?.stateOverrides &&
        Object.keys(simulationData.stateOverrides).length > 0 ? (
          <div className="space-y-2 py-2">
            <div className="text-ilabel text-gray bg-gray-50 px-2 py-1 font-medium dark:bg-gray-800">
              State Overrides
            </div>
            <div className="space-y-2 px-2">
              {Object.entries(simulationData.stateOverrides).map(
                ([address, overrides]) => (
                  <div
                    key={address}
                    className="flex place-items-stretch space-x-2"
                  >
                    <ContractAddress address={address} />
                    <ArrowRightCircleIcon className="h-4 w-4 text-gray-400" />
                    <div>
                      {overrides.state &&
                      Object.keys(overrides.state).length > 0 ? (
                        <div className="flex items-center">
                          <div className="text-gray mr-2">State</div>
                          <div className="text-ilabel text-primary-800/80 dark:text-primary-200/80">
                            {JSON.stringify(overrides.state)}
                          </div>
                        </div>
                      ) : null}
                      {overrides.balance ? (
                        <div className="flex items-center">
                          <div className="text-gray mr-2 inline-block">
                            Balance
                          </div>
                          <div className="text-ilabel text-primary-800/80 dark:text-primary-200/80 inline-block">
                            {BD(overrides.balance).toString()}
                          </div>
                        </div>
                      ) : null}
                      {overrides.nonce ? (
                        <div className="flex items-center">
                          <div className="text-gray mr-2 inline-block">
                            Nonce
                          </div>
                          <div className="text-ilabel text-primary-800/80 dark:text-primary-200/80 inline-block">
                            {overrides.nonce}
                          </div>
                        </div>
                      ) : null}
                      {overrides.code ? (
                        <div className="flex items-center">
                          <div className="text-gray mr-2 inline-block">
                            Code
                          </div>
                          <div className="text-ilabel text-primary-800/80 dark:text-primary-200/80 inline-block font-mono text-xs">
                            {overrides.code.slice(0, 20)}...
                          </div>
                        </div>
                      ) : null}
                    </div>
                  </div>
                )
              )}
            </div>
          </div>
        ) : null}
      </div>
    </Collapse>
  )
}

export default SimulatorInfo
