import React, { useContext } from 'react'
import { CopyButton } from '../common/CopyButton'
import Avatar from 'boring-avatars'
import { getChainExternalUrl, getChainBlockscoutUrl, getSuiscanUrl } from '@sentio/chain'
import { useResizeDetector } from 'react-resize-detector'
import { chainIdToNumber, getPathHostName, useAddressTag } from '../utils/use-tag'
import { cva, cx as classNames } from 'class-variance-authority'
import { ExternalLogo } from './ExternalLogo'
import { SvgFolderContext } from '../utils/extension-context'

const iconClassName = cva('', {
  variants: {
    size: {
      sm: '!w-4 !h-4',
      lg: '!w-5 !h-5'
    }
  },
  defaultVariants: {
    size: 'sm'
  }
})

interface Props {
  data: string
  className?: string
  truncate?: number
  autoTruncate?: boolean
  copyable?: boolean
  avatar?: boolean
  avatarProps?: React.ComponentProps<typeof Avatar>
  chainId?: string
  type?: 'address' | 'tx' | 'block'
  trigger?: 'hover' | 'static'
  noCopyHint?: boolean // for performance reason, we could disable the copy hint
  showTag?: boolean
  link?: string
  size?: 'sm' | 'lg'
  noLink?: boolean
}

export const HexNumber = React.memo(function HexNumber({
  data,
  className,
  truncate: _truncate,
  autoTruncate,
  copyable,
  avatar,
  avatarProps,
  chainId,
  type,
  trigger = 'hover',
  noCopyHint = true,
  showTag,
  link,
  size = 'sm',
  noLink = false
}: Props) {
  let displayString = data
  let title: string | undefined = undefined
  const { width, ref } = useResizeDetector({
    handleHeight: false,
    refreshMode: 'throttle',
    refreshRate: 1000
  })
  const { data: tagData } = useAddressTag(showTag ? data : undefined)
  const folderPath = useContext(SvgFolderContext)

  if (data === undefined || data === null) {
    return null
  }

  const externalLink = link || getChainExternalUrl(chainIdToNumber(chainId), data, type)
  const blockscoutLink = getChainBlockscoutUrl(chainIdToNumber(chainId), data, type)
  const suiscanLink = getSuiscanUrl(chainId, data, type)

  let truncate = _truncate
  if (autoTruncate && width && width > 0) {
    let totalWidth = width - 8
    if (externalLink) {
      totalWidth = totalWidth - 24
    }
    if (blockscoutLink) {
      totalWidth = totalWidth - 24
    }
    if (copyable) {
      totalWidth -= 24
    }
    if (avatar) {
      totalWidth -= 16
    }
    truncate = Math.max(8, Math.floor(totalWidth / 8))
  }
  if (truncate && data.length > truncate) {
    displayString = `${data.substring(0, truncate - 4)}...${data.substring(data.length - 4)}`
    title = data
  }
  if (tagData?.primaryName) {
    displayString = tagData.primaryName
  }

  return (
    <span
      className={classNames(
        'number items-center gap-2',
        className,
        copyable && 'group flex-nowrap',
        autoTruncate ? 'flex w-full' : 'inline-flex'
      )}
      ref={ref}
    >
      {avatar ? (
        <Avatar
          size={16}
          name={data.startsWith('0x') ? data.substring(2) : data}
          variant="pixel"
          colors={['#92A1C6', '#146A7C', '#F0AB3D', '#C271B4', '#C20D90']}
          {...avatarProps}
        />
      ) : null}
      <span title={title}>{displayString}</span>
      {copyable ? (
        <div
          onClick={(e) => e.stopPropagation()}
          className={
            trigger === 'hover'
              ? 'invisible opacity-0 transition-opacity duration-300 ease-in-out group-hover:visible group-hover:opacity-100'
              : ''
          }
        >
          <CopyButton text={data} size={size === 'lg' ? 20 : 18} />
        </div>
      ) : null}
      {!noLink && externalLink ? (
        <a
          href={externalLink}
          target="_blank"
          rel="noreferrer"
          title={getPathHostName(externalLink) ? `Open in ${getPathHostName(externalLink)}` : 'Open in external link'}
          className="w-max shrink-0"
        >
          <ExternalLogo
            className={classNames(
              'hover:text-primary active:text-primary-700',
              trigger === 'static' ? '' : 'collapse group-hover:visible',
              iconClassName({ size })
            )}
            link={externalLink}
          />
        </a>
      ) : null}
      {!noLink && blockscoutLink ? (
        // eslint-disable-next-line react/jsx-no-target-blank -- blockscout need track sentio referrer
        <a
          href={blockscoutLink}
          target="_blank"
          title="Open in Blockscout"
          className="w-max shrink-0 rounded-full hover:ring-2"
        >
          <img
            src={`${folderPath}/blockscout-logo.png`}
            alt="Blockscout Logo"
            className={classNames(
              'hover:text-primary active:text-primary-700 rounded-full',
              trigger === 'static' ? '' : 'collapse group-hover:visible',
              iconClassName({ size })
            )}
          />
        </a>
      ) : null}
      {!noLink && suiscanLink && suiscanLink !== externalLink ? (
        <a href={suiscanLink} target="_blank" rel="noreferrer" title="Open in Scan" className="w-max shrink-0">
          <ExternalLogo
            className={classNames(
              'hover:text-primary active:text-primary-700',
              trigger === 'static' ? '' : 'collapse group-hover:visible',
              iconClassName({ size })
            )}
            link={suiscanLink}
          />
        </a>
      ) : null}
    </span>
  )
})
