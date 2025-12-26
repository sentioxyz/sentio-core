import { useMemo } from 'react'
import dayjs from 'dayjs'
import localizedFormat from 'dayjs/plugin/localizedFormat'
import relativeTime from 'dayjs/plugin/relativeTime'
import { useEthScription } from '~/content/lib/ethscriptions/use-ethscription'
import { ethscriptionUrl } from '~/utils/url'
import { classNames } from '@sentio/ui-web3'
import { ESPContent } from './InscriptionContent'

dayjs.extend(localizedFormat)
dayjs.extend(relativeTime)

interface Props {
  hash?: string
  className?: string
  displayItem?: boolean
}

export const ESPItem = ({ hash, className, displayItem }: Props) => {
  const { data: ethData, loading } = useEthScription(hash)
  const title = useMemo(() => {
    let title = ''
    if (ethData?.collection_items && ethData.collection_items.length > 0) {
      title = `${ethData.collection_items[0]?.name}`
    } else if (ethData?.ethscription_number) {
      title = `Ethscription #${ethData?.ethscription_number}`
    } else {
      title = 'Ethscription Item'
    }
    return title
  }, [ethData])

  let node: any
  if (loading) {
    node = (
      <div
        className={classNames('flex animate-pulse flex-col gap-5', className)}
      >
        <div className="rounded bg-gray-500" style={{ height: 36 }}></div>
        {!displayItem && <div className="-mt-2 flex flex-col gap-3"></div>}
        <div className="rounded-xl bg-slate-50 px-4 py-2 shadow">
          <div className="divide-y divide-slate-200">
            <div className="space-y-2 py-2">
              <div className="flex justify-between">
                <div className="text-gray text-sm font-bold">Owner</div>
              </div>
              <div className="h-5 rounded bg-gray-500"></div>
            </div>
            <div className="space-y-2 py-2">
              <div className="flex justify-between">
                <div className="text-gray text-sm font-bold">Creator</div>
              </div>
              <div className="h-5 rounded bg-gray-500"></div>
            </div>
            <div className="space-y-2 py-2">
              <div className="flex justify-between">
                <div className="text-gray text-sm font-bold">Create Date</div>
              </div>
              <div className="h-5 rounded bg-gray-500"></div>
            </div>
            <div className="space-y-2 py-2">
              <div className="flex justify-between">
                <div className="text-gray text-sm font-bold">ESIP6</div>
              </div>
              <div className="h-5 rounded bg-gray-500"></div>
            </div>
          </div>
        </div>
      </div>
    )
  } else {
    node = (
      <div className={classNames('flex flex-col gap-5', className)}>
        <h1 className="text-lg leading-none" style={{ fontWeight: 900 }}>
          <a
            href={ethscriptionUrl(hash!)}
            target="_blank"
            rel="noreferrer"
            className="hover:text-primary hover:underline"
          >
            {title}
          </a>
        </h1>
        {!displayItem && <div className="-mt-2 flex flex-col gap-3"></div>}
        <div className="rounded-xl bg-slate-50 px-4 py-2 shadow">
          <div className="divide-y divide-slate-200">
            <div className="space-y-2 py-2">
              <div className="flex justify-between">
                <div className="text-gray text-sm font-bold">Owner</div>
              </div>
              <a
                className="cursor-pointer break-words font-mono text-sm text-slate-800 hover:underline"
                href={`https://ethscriptions.com/${ethData?.current_owner}`}
                target="_blank"
                rel="noreferrer"
              >
                {ethData?.current_owner}
              </a>
            </div>
            <div className="space-y-2 py-2">
              <div className="flex justify-between">
                <div className="text-gray text-sm font-bold">Creator</div>
              </div>
              <a
                className="cursor-pointer break-words font-mono text-sm text-slate-800 hover:underline"
                href={`https://ethscriptions.com/${ethData?.creator}`}
                target="_blank"
                rel="noreferrer"
              >
                {ethData?.creator}
              </a>
            </div>
            <div className="space-y-2 py-2">
              <div className="flex justify-between">
                <div className="text-gray text-sm font-bold">Create Date</div>
              </div>
              <div className="break-words text-sm text-slate-800">
                {ethData?.creation_timestamp
                  ? dayjs(ethData?.creation_timestamp).format('LLL')
                  : 'N/A'}
                {ethData?.creation_timestamp
                  ? ` (${dayjs(ethData?.creation_timestamp).fromNow()})`
                  : null}
              </div>
            </div>
            <div className="space-y-2 py-2">
              <div className="flex justify-between">
                <div className="text-gray text-sm font-bold">ESIP6</div>
              </div>
              <div className="break-words text-sm text-slate-800">
                {ethData?.esip6?.toString()}
              </div>
            </div>
          </div>
        </div>
      </div>
    )
  }

  if (displayItem) {
    return (
      <div className="flex gap-8">
        <div style={{ width: 300 }}>
          {ethData?.content_uri ? (
            <ESPContent
              contentURI={ethData.content_uri}
              index={ethData?.ethscription_number}
            />
          ) : (
            <div className="h-full w-full animate-pulse rounded bg-gray-500"></div>
          )}
        </div>
        {node}
      </div>
    )
  }

  return node
}
