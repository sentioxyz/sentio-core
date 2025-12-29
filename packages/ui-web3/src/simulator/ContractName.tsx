import { useSimulatorContext } from './SimulatorContext'
import { CheckBadgeIcon } from '@heroicons/react/24/outline'
import Avatar from 'boring-avatars'

export const AddressAvatar = (props: any) => (
  <Avatar
    size={16}
    variant="pixel"
    colors={['#92A1C6', '#146A7C', '#F0AB3D', '#C271B4', '#C20D90']}
    {...props}
  />
)

export const ContractName = ({ address }: { address?: string }) => {
  const { contractName: name } = useSimulatorContext()
  return (
    <div className="text-icontent flex w-full items-center justify-between rounded-md bg-gray-50 px-2 py-1.5 dark:bg-gray-100">
      <span className="inline-flex items-center gap-2">
        {name ? <AddressAvatar size={24} name={address} /> : null}
        <span className={name ? 'text-primary' : 'text-gray-400'}>
          {name || `Can not get name of target contract "${address}"`}
        </span>
      </span>
      {name ? (
        <span className="inline-flex items-center">
          <CheckBadgeIcon className="h-4 w-4 text-cyan-800" />
        </span>
      ) : null}
    </div>
  )
}
