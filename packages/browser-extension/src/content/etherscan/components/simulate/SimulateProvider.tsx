import {
  contractAddress,
  contractNetwork,
  chainIdentifier,
  compilationId,
  blockNumber,
  simId,
  simBundleId,

  // derived atoms
  contractName,
  contractFunctions,
  latestBlockNumber,
  blockSummary
} from '@sentio/ui-web3'
import { useSetAtom, useAtomValue, Provider } from 'jotai/react'
import { useEffect } from 'react'

async function sendRequest(apiName: string, request: any, initReq?: any) {
  const res = await chrome.runtime.sendMessage({
    api: apiName,
    data: {
      ...request,
      ...initReq
    }
  })
  if (res.code && res.message) {
    throw {
      body: {
        ...res
      }
    }
  }
  return res
}

function InterSimulateProvider({ children }: { children?: React.ReactNode }) {
  const _contractAddress = useAtomValue(contractAddress)
  const _contractNetwork = useAtomValue(contractNetwork)
  const _chainIdentifier = useAtomValue(chainIdentifier)
  const _compilationId = useAtomValue(compilationId)
  const _blockNumber = useAtomValue(blockNumber)
  const _simId = useAtomValue(simId)
  const _simBundleId = useAtomValue(simBundleId)

  const setContractName = useSetAtom(contractName)
  const setContractFunctions = useSetAtom(contractFunctions)
  const setLatestBlockNumber = useSetAtom(latestBlockNumber)
  const setBlockSummary = useSetAtom(blockSummary)

  // contractName
  useEffect(() => {
    ;(async () => {
      if (_contractAddress && _contractNetwork && _chainIdentifier) {
        try {
          const res = await sendRequest('GetContractName', {
            address: _contractAddress,
            [_chainIdentifier]: _contractNetwork
          })
          setContractName(res.contractName)
        } catch {
          setContractName('')
        }
      }
    })()
  }, [_contractAddress, _contractNetwork, _chainIdentifier])

  // contractFunctions
  useEffect(() => {
    ;(async () => {
      if (_contractAddress || !_contractNetwork) {
        setContractFunctions({})
        return
      }

      try {
        let parsedABI: any[] = []
        const res = await sendRequest(
          'GetABI',
          _simBundleId
            ? {
                address: _contractAddress,
                [_chainIdentifier]: _contractNetwork,
                txId: {
                  bundleId: _simBundleId
                }
              }
            : _simId
              ? {
                  address: _contractAddress,
                  [_chainIdentifier]: _contractNetwork,
                  txId: {
                    simulationId: _simId
                  }
                }
              : {
                  address: _contractAddress,
                  [_chainIdentifier]: _contractNetwork
                }
        )
        const { ABI, name } = res as any
        parsedABI = JSON.parse(ABI)

        const functions = parsedABI.filter((item) => item.type === 'function')
        const wfunctions = functions.filter(
          (item) =>
            item.stateMutability === 'payable' ||
            item.stateMutability === 'nonpayable'
        )
        const rfunctions = functions.filter(
          (item) =>
            item.stateMutability === 'pure' || item.stateMutability === 'view'
        )
        setContractFunctions({
          wfunctions,
          rfunctions,
          name
        })
      } catch {
        setContractFunctions({})
      }
    })()
  }, [
    _contractAddress,
    _contractNetwork,
    _chainIdentifier,
    _simId,
    _simBundleId,
    _compilationId
  ])

  // latestBlockNumber
  useEffect(() => {
    ;(async () => {
      if (!_contractNetwork || !_chainIdentifier) {
        setLatestBlockNumber({})
        return
      }

      try {
        const res = await sendRequest('GetLatestBlockNumber', {
          [_chainIdentifier]: _contractNetwork
        })
        setLatestBlockNumber(res)
      } catch {
        setLatestBlockNumber({})
      }
    })()
  }, [_contractNetwork, _chainIdentifier])

  // blockSummary
  useEffect(() => {
    ;(async () => {
      if (!_blockNumber || !_contractNetwork || !_chainIdentifier) {
        setBlockSummary({})
        return
      }

      try {
        const res = await sendRequest('GetBlockSummary', {
          blockNumber: _blockNumber.toString(),
          [_chainIdentifier]: _contractNetwork
        })
        setBlockSummary(res)
      } catch {
        setBlockSummary({})
      }
    })()
  }, [_blockNumber, _contractNetwork, _chainIdentifier])

  return children
}

export function SimulateProvider({ children }: { children?: React.ReactNode }) {
  return (
    <Provider>
      <InterSimulateProvider>{children}</InterSimulateProvider>
    </Provider>
  )
}
