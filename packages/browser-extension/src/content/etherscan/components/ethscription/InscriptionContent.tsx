import { parseDataURI } from '~/content/lib/ethscriptions/util'

export const ESPContent = ({
  contentURI,
  index
}: {
  contentURI: string
  index?: string
}) => {
  const data = parseDataURI(contentURI)
  if (!data.isValid) {
    return null
  }

  if (data?.mediaType?.startsWith('image')) {
    return (
      <div className="max-h-[100%] grow overflow-hidden rounded-xl border bg-slate-50 text-sm shadow-sm">
        <div className="flex items-baseline justify-between bg-white p-3">
          <p className="font-mono text-sm">{data.mimeType}</p>
        </div>
        <img
          className="w-full"
          src={`data:${data.mimeType};${data.base64},${data.data}`}
          alt={`Ethscription#${index}`}
          style={{
            imageRendering: 'pixelated'
          }}
        />
      </div>
    )
  }
  try {
    const json = JSON.parse(data?.data || '')
    return (
      <div className="max-h-[100%] grow overflow-hidden rounded-xl border bg-slate-50 text-sm shadow-sm">
        <div className="flex items-baseline justify-between bg-white p-3">
          <p className="font-mono text-sm">{data.mimeType}</p>
        </div>
        <pre className="whitespace-pre-wrap">
          {json ? JSON.stringify(json, null, 2) : null}
        </pre>
      </div>
    )
  } catch {
    //ignore
  }
  return null
}
