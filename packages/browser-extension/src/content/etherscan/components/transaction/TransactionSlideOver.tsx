import Drawer from '@mui/material/Drawer'
import { TransactionCard } from './TransactionCard'
import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  IsSimulationContext,
  OpenSimulationContext
} from '~/content/lib/context/transaction'
import { Simulation } from '~/content/lib/types/simulation'
import {
  sentioCompilationUrl,
  sentioProjectSimUrl,
  sentioSimUrl
} from '~/utils/url'
import { SimulateHistory } from '../simulate/SimulateHistory'
import {
  SvgFolderContext,
  OpenCompilationContext,
  CopyButton,
  GlobalQueryContext,
  classNames
} from '@sentio/ui-web3'

const slideOverKey = 'transaction.slideOver.isExpand'

function shorterHash(hash: string) {
  if (!hash || hash.length < 12) {
    return hash
  }
  return `${hash.substring(0, 8)}...${hash.substring(hash.length - 4)}`
}

export const TransactionSlideOver = () => {
  const [visible, setVisible] = useState(false)
  const [isExpand, setExpand] = useState(false)
  const [hash, setHash] = useState('')
  const [simId, setSimId] = useState('')
  const [chainId, setChainId] = useState('')

  const [simulations, setSimulations] = useState<
    { simulation: Simulation; projectOwner?: string; projectSlug?: string }[]
  >([])
  const [isSimulation, setIsSimulation] = useState<boolean>(false)

  const openSimulation = useCallback((res) => {
    const { simulation, projectOwner, projectSlug } = res
    if (simulation.id) {
      setSimulations((pre) => [...pre, res])
      setIsSimulation(true)
      setSimId(simulation.id)
    }
    return ''
  }, [])

  const currentSim = useMemo(() => {
    return simulations.find((item) => item.simulation.id === simId)
  }, [simId, simulations])

  useEffect(() => {
    chrome.storage.local.get([slideOverKey], function (result) {
      setExpand(result[slideOverKey] as boolean)
    })
  }, [])

  useEffect(() => {
    const global = window as any
    global.openSlideOver = (hash: string, chainId: string) => {
      setVisible(true)
      setHash(hash)
      setIsSimulation(false)
      setSimulations([])
      setChainId(chainId)
    }

    return () => {
      delete global.openSlideOver
    }
  }, [])

  const simulationLink = useMemo(() => {
    const targetSimulation = simulations.find(
      (item) => item.simulation.id === simId
    )
    if (targetSimulation) {
      if (targetSimulation.projectOwner && targetSimulation.projectSlug) {
        return sentioProjectSimUrl(
          chainId,
          simId,
          targetSimulation.projectOwner,
          targetSimulation.projectSlug
        )
      }
      return sentioSimUrl(chainId, simId)
    }
  }, [simulations, simId, chainId])

  const simulationList = useMemo(() => {
    return simulations
      .map((simulation) => ({
        id: simulation.simulation.id,
        name: simulation.simulation.id,
        createdAt: simulation.simulation.createAt
      }))
      .reverse()
  }, [simulations])

  const queryContext = useMemo(() => {
    const targetSim = simulations.find((item) => item.simulation.id === simId)
    if (targetSim?.projectOwner && targetSim?.projectSlug) {
      return {
        owner: targetSim.projectOwner,
        slug: targetSim.projectSlug
      }
    }
    return {} as Record<string, string>
  }, [simId, simulations])

  return (
    <SvgFolderContext.Provider value="https://app.sentio.xyz/">
      <Drawer
        anchor="right"
        open={visible}
        onClose={() => {
          setVisible(false)
        }}
        // hideBackdrop
        className="_sentio_"
        classes={{
          paper: 'text-gray-600'
        }}
      >
        <div
          className="dark:bg-sentio-gray-100 bg-white transition-all duration-300 ease-in-out"
          key={visible ? hash + '_open' : hash + '_hidden'}
          style={{
            width: isExpand ? '80vw' : '800px'
          }}
        >
          <div
            className="dark:bg-sentio-gray-100 sticky top-0 flex w-full justify-between border-b border-gray-100 bg-white px-4 py-2"
            style={{
              zIndex: 10,
              backgroundColor: 'white'
            }}
          >
            <div className="text-dt inline-flex items-center gap-2">
              <button
                className="text-gray py-1 text-xs"
                onClick={() => {
                  const newValue = !isExpand
                  setExpand(newValue)
                  chrome.storage.local.set({ [slideOverKey]: newValue })
                }}
              >
                <i
                  className={classNames(
                    'text-gray mr-2 inline h-4 w-4 align-text-top transition-transform',
                    isExpand
                      ? 'fas fa-solid fa-chevron-right'
                      : 'fas fa-solid fa-chevron-left'
                  )}
                ></i>
                {isExpand ? 'Collapse' : 'Expand'}
              </button>
              {isSimulation ? (
                <span className="inline-flex items-center rounded bg-orange-100 px-2 py-0.5 text-sm font-medium text-orange-800">
                  Simulation
                </span>
              ) : null}
              <a
                href={isSimulation ? simulationLink : `/tx/${hash}`}
                className="text-dt cursor-pointer font-medium hover:underline"
                target="_blank"
                rel="noreferrer"
              >
                {isSimulation ? simId : hash}
              </a>
              <span className="relative">
                <CopyButton text={isSimulation ? simId : hash} size={16} />
              </span>
              {isSimulation ? (
                <span
                  title={`raw transaction ${hash}`}
                  className="inline-flex items-center gap-1 text-xs text-gray-400"
                >
                  <span>( Fork from </span>
                  <a
                    className="text-primary-400 cursor-pointer hover:underline"
                    href={`/tx/${hash}`}
                    target="_blank"
                    rel="noreferrer"
                  >
                    {shorterHash(hash)}
                  </a>
                  <CopyButton text={hash} size={16} />
                  <span>)</span>
                </span>
              ) : null}
            </div>
            <div className="flex items-center gap-2">
              {simulationList.length > 0 ? (
                <SimulateHistory
                  simulates={simulationList}
                  value={simId}
                  onChange={(value) => {
                    setSimId(value)
                  }}
                />
              ) : null}
              <div
                className="hover:text-primary cursor-pointer text-lg"
                onClick={() => {
                  setVisible(false)
                }}
              >
                <i className="fa-solid fa-xmark fal fa-times"></i>
              </div>
            </div>
          </div>
          <div className="px-4 pt-2 text-xs">
            <OpenCompilationContext.Provider
              value={(id: string) => {
                if (id) {
                  window.open(sentioCompilationUrl(id), '_blank')
                }
              }}
            >
              <IsSimulationContext.Provider value={isSimulation}>
                <OpenSimulationContext.Provider value={openSimulation}>
                  <GlobalQueryContext.Provider value={queryContext}>
                    <TransactionCard
                      hash={isSimulation ? simId : hash}
                      chainId={chainId}
                      defaultShowCallTrace={true}
                      showBrief={true}
                      projectOwner={currentSim?.projectOwner}
                      projectSlug={currentSim?.projectSlug}
                    />
                  </GlobalQueryContext.Provider>
                </OpenSimulationContext.Provider>
              </IsSimulationContext.Provider>
            </OpenCompilationContext.Provider>
          </div>
        </div>
      </Drawer>
    </SvgFolderContext.Provider>
  )
}
