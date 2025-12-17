import React from 'react'
import { LuExternalLink } from 'react-icons/lu'
import { cx as classNames } from 'class-variance-authority'
import { ChainIconProps, getChainIconFactory } from './ChainIcons'
import { SvgFolderContext } from '../utils/extension-context'
import { useContext } from 'react'
import { useDarkMode } from '../utils/extension-context'
import { EthChainId } from '@sentio/chain'

const PolygonChainIcon = getChainIconFactory(EthChainId.POLYGON)
const LineaIcon = getChainIconFactory(EthChainId.LINEA)

const scanIconMap = {
  polygonScan: {
    icon: PolygonChainIcon,
    urlRegex: /^https:\/\/polygonscan\.com/
  },
  lineaScan: {
    icon: LineaIcon,
    urlRegex: /^https:\/\/lineascan\.build/
  },
  etherScan: {
    icon: (props: ChainIconProps) => {
      const folderPath = useContext(SvgFolderContext)
      const isDarkMode = useDarkMode()
      return (
        <img
          src={`${folderPath}/${isDarkMode ? 'etherscan-logo-circle-light.svg' : 'etherscan-logo-circle.svg'}`}
          alt="Etherscan Logo"
          {...props}
        />
      )
    },
    urlRegex: /^https:\/\/[\w-]*\.?etherscan\.io/
  },
  bscScan: {
    icon: (props: ChainIconProps) => {
      const folderPath = useContext(SvgFolderContext)
      return <img src={`${folderPath}/bscscan-logo-circle.svg`} alt="Etherscan Logo" {...props} />
    },
    urlRegex: /^https:\/\/bscscan\.com/
  },
  blockscout: {
    icon: (props: ChainIconProps) => {
      const folderPath = useContext(SvgFolderContext)
      return <img src={`${folderPath}/blockscout-logo.png`} alt="Blockscout Logo" {...props} />
    },
    urlRegex: /^https:\/\/[\w-]+\.?blockscout\.com/
  },
  aptosExplorer: {
    icon: (props: ChainIconProps) => {
      const isDarkMode = useDarkMode()
      const folderPath = useContext(SvgFolderContext)
      return <img src={`${folderPath}/${isDarkMode ? 'aptos-dark.svg' : 'aptos.svg'}`} alt="Aptos Logo" {...props} />
    },
    urlRegex: /^https:\/\/explorer\.aptoslabs\.com/
  },
  suiscan: {
    icon: (props: ChainIconProps) => {
      const folderPath = useContext(SvgFolderContext)
      return <img src={`${folderPath}/suiscan.svg`} alt="Suiscan Logo" {...props} />
    },
    urlRegex: /^https:\/\/suiscan\.xyz/
  }
  // suivision: {
  //   icon: (props) => {
  //     const folderPath = useContext(SvgFolderContext)
  //     return <img src={`${folderPath}/suivision.svg`} alt="Suivision Logo" {...props} />
  //   },
  //   urlRegex: /^https:\/\/suivision\.xyz/
  // }
}

export const ExternalLogo = ({ link, className }: { link?: string; className?: string }) => {
  if (!link) {
    return <span className={className} />
  }

  const scanIcon = Object.values(scanIconMap).find(({ urlRegex }) => urlRegex.test(link))
  if (scanIcon) {
    return React.createElement(scanIcon.icon, { className: classNames(className, 'rounded-full hover:ring-2') })
  }
  return <LuExternalLink className={className} />
}
