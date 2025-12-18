import BarLoader from 'react-spinners/BarLoader'
import { LoaderHeightWidthProps } from 'react-spinners/helpers/props'
import { memo } from 'react'
import { cx as classNames } from 'class-variance-authority'

interface Props {
  hint?: React.ReactNode
  loading?: boolean
  className?: string
  iconClassName?: string
  width?: LoaderHeightWidthProps['width']
  gray?: boolean
}

function _BarLoading({ hint = 'Loading Sentio', loading = true, className, iconClassName, width = 150, gray }: Props) {
  if (loading) {
    return (
      <div className={classNames('loading-container flex h-full flex-col justify-center overflow-hidden', className)}>
        {hint && <div className="loading-text text-icontent text-gray my-2 text-center font-medium">{hint}</div>}
        <div className="flex justify-center pt-1">
          <BarLoader
            color="#0756D5"
            loading={true}
            height={5}
            width={width}
            cssOverride={{
              borderRadius: '4px'
            }}
          />
        </div>
      </div>
    )
  }
  return null
}

export const BarLoading = memo(_BarLoading)

export default BarLoading
