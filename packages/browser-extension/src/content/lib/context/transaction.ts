import { Simulation } from '~/content/lib/types/simulation'
import { createContext } from 'react'
import { sentioProjectSimUrl, sentioSimUrl } from '~/utils/url'

export const OpenSimulationContext = createContext(
  (res: {
    simulation: Simulation
    projectOwner?: string
    projectSlug?: string
  }) => {
    let link = ''
    if (res.projectOwner && res.projectSlug) {
      link = sentioProjectSimUrl(
        res.simulation.networkId!,
        res.simulation.id!,
        res.projectOwner,
        res.projectSlug
      )
    } else {
      link = sentioSimUrl(res.simulation.networkId, res.simulation.id)
    }
    if (link) {
      window.open(link, '_blank')
    }
    return link
  }
)

export const IsSimulationContext = createContext(false)
