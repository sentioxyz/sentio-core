import { memo, useContext } from 'react'
import { getChainExternalUrl, getChainBlockscoutUrl } from '@sentio/chain'
import { cx as classNames } from 'class-variance-authority'
import { chainIdToNumber, getPathHostName } from '../utils/use-tag'
import { ExternalLogo } from './ExternalLogo'
import { SvgFolderContext } from '@sentio/ui-core'

const EtherLink = ({
  address,
  chainId,
  link,
  type = 'address',
  trigger
}: {
  address?: string
  chainId?: string
  link?: string
  type?: 'address' | 'tx' | 'block' | 'token'
  trigger?: 'static' | 'hover'
}) => {
  const chainIdNumber = chainIdToNumber(chainId)
  const externalLink = getChainExternalUrl(chainIdNumber, address, type) || link
  const blockscoutLink = getChainBlockscoutUrl(chainIdNumber, address, type)
  const folderPath = useContext(SvgFolderContext)

  if (!externalLink && !blockscoutLink) {
    return null
  }

  return (
    <>
      <a
        href={externalLink}
        target="_blank"
        rel="noreferrer"
        className="inline-block"
        title={getPathHostName(externalLink)}
      >
        <ExternalLogo
          className={classNames('h-4 w-4', trigger === 'static' ? '' : 'collapse group-hover:visible')}
          link={externalLink}
        />
      </a>
      {blockscoutLink ? (
        // eslint-disable-next-line react/jsx-no-target-blank -- blockscout need track sentio referrer
        <a href={blockscoutLink} target="_blank" className="inline-block" title="Blockscout">
          <img
            className={classNames(
              'h-4 w-4 rounded-full hover:ring-2',
              trigger === 'static' ? '' : 'collapse group-hover:visible'
            )}
            src={`${folderPath}/blockscout-logo.png`}
            alt="Blockscout Icon"
          />
        </a>
      ) : null}
    </>
  )
}

export default memo(EtherLink)
