import { DataURI } from '~/content/lib/ethscriptions/util'
import { ESPItem } from './InscriptionItem'

const DataDisplay = ({
  data,
  index
}: {
  data?: DataURI
  index?: string | number
}) => {
  if (data?.mediaType?.startsWith('image')) {
    return (
      <img
        className="w-full"
        src={`data:${data.mimeType};${data.base64},${data.data}`}
        alt={`Ethscription#${index}`}
        style={{
          imageRendering: 'pixelated'
        }}
      />
    )
  }
  try {
    const json = JSON.parse(data?.data || '')
    return (
      <pre className="whitespace-pre-wrap">
        {json ? JSON.stringify(json, null, 2) : null}
      </pre>
    )
  } catch {
    //ignore
  }
  return null
}

interface Props {
  data?: DataURI
  hash?: string
}

export const EthscriptionCreate = ({ data, hash }: Props) => {
  if (!data || !data.isValid) {
    return null
  }

  return (
    <div className="space-y-4">
      <h3 className="text-lg font-bold text-slate-800">Mint Ethscriptions</h3>
      <div className="flex items-stretch gap-12">
        <div className="flex min-w-0 grow flex-col gap-7">
          <div className="max-h-[100%] grow overflow-hidden rounded-xl border bg-slate-50 text-sm shadow-sm">
            <div className="flex items-baseline justify-between bg-white p-3">
              <p className="font-mono text-sm">{data.mimeType}</p>
            </div>
            <DataDisplay data={data} index={hash} />
          </div>
        </div>
        <ESPItem hash={hash} className="w-1/2 flex-shrink-0" />
      </div>
    </div>
  )
}
