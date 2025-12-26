import { ESPItem } from './InscriptionItem'

interface Props {
  transferList?: string[]
}

export const EthscriptionTransfer = ({ transferList }: Props) => {
  return (
    <div className="space-y-2">
      <h3 className="text-lg font-bold text-slate-800">
        Transferring Ethscriptions ({transferList?.length} item
        {transferList && transferList?.length > 1 ? 's' : ''})
      </h3>
      <div className="w-full overflow-auto">
        <div className="flex w-fit min-w-full gap-8 py-4">
          {transferList?.map((hash, index) => (
            <div
              className="w-[600px] rounded-xl border p-4 shadow 2xl:w-[800px]"
              key={index}
            >
              <ESPItem hash={hash} className="w-full" displayItem={true} />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
