import { useMemo } from 'react'
import { getChainExternalUrl } from '@sentio/chain'
import { cx as classNames } from 'class-variance-authority'
import { CopyButton } from '@sentio/ui-core'
import { ExternalLogo } from '../transaction/ExternalLogo'

interface Props {
  data?: string
  type?: 'block' | 'tx' | 'address'
  className?: string
  externalLogoClassName?: string
  trigger?: 'hover' | 'static'
  truncate?: boolean
  chainId?: string
}

const getPathHostName = (url?: string): string => {
  if (!url) return ''
  try {
    return new URL(url).hostname
  } catch {
    return ''
  }
}

export const MevLink = ({
  data,
  type,
  className,
  trigger,
  truncate,
  externalLogoClassName,
  chainId
}: Props) => {
  if (data === undefined || data === null) {
    return null
  }

  const externalLink = chainId
    ? getChainExternalUrl(chainId, data, type)
    : undefined
  const hashStr = useMemo(() => {
    if (!data || !type) {
      return ''
    }
    if (type === 'block') {
      return data
    }
    return truncate ? data.slice(0, 6) + '...' + data.slice(-4) : data
  }, [data, type, truncate])

  const innerLink = useMemo(() => {
    if (
      typeof window === 'undefined' ||
      type === 'block' ||
      !data ||
      !chainId
    ) {
      return undefined
    }
    const url = new URL(window.location.href)
    const hostname = window.location.hostname
    if (hostname !== 'localhost' && !hostname.endsWith('sentio.xyz')) {
      url.host = 'app.sentio.xyz'
    }
    url.pathname =
      type === 'tx' ? `/tx/${chainId}/` + data : `/contract/${chainId}/` + data
    return url.toString()
  }, [data, type, chainId])

  return (
    <div
      className={classNames(
        'group inline-flex items-center gap-2 font-mono',
        className
      )}
    >
      <a
        href={innerLink}
        target="_blank"
        rel="noreferrer"
        className={classNames(innerLink ? 'hover:underline' : '', 'text-xs')}
      >
        {hashStr}
      </a>
      <div
        onClick={(e) => e.stopPropagation()}
        className={trigger === 'static' ? '' : 'invisible group-hover:visible'}
      >
        <CopyButton text={data} size={16} className="hover:text-primary" />
      </div>
      {externalLink && (
        <a
          href={externalLink}
          target="_blank"
          rel="noreferrer"
          title={getPathHostName(externalLink)}
          className="w-max shrink-0"
        >
          <ExternalLogo
            className={classNames(
              'hover:text-primary active:text-primary-700',
              trigger === 'static' ? '' : 'invisible group-hover:visible',
              '!h-4 !w-4',
              externalLogoClassName
            )}
            link={externalLink}
          />
        </a>
      )}
    </div>
  )
}
