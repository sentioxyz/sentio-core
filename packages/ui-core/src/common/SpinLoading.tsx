import React from 'react'
import ClipLoader from 'react-spinners/ClipLoader'
import { cx as classNames } from 'class-variance-authority'

interface Props {
  loading?: boolean
  children?: React.ReactNode
  className?: string
  size?: number
  showMask?: boolean
  maskOpacity?: number // 0-100
}

export const SpinLoading = React.forwardRef<HTMLDivElement, Props>(function Spinner(args: Props, ref) {
  const { loading = false, children, className, size = 48, showMask, maskOpacity = 80 } = args
  return (
    <div className={classNames('relative', className)}>
      {showMask && loading && (
        <div
          className={classNames(
            'absolute bottom-0 left-0 right-0 top-0 z-[1]',
            maskOpacity ? `bg-white dark:bg-sentio-gray-100/${maskOpacity}` : 'dark:bg-sentio-gray-100 bg-white'
          )}
        ></div>
      )}
      <div className="absolute left-[50%] top-[50%] z-[1] -translate-y-6">
        <ClipLoader
          loading={loading}
          color="#3B82F6"
          size={size}
          cssOverride={{
            borderWidth: 3
          }}
        />
      </div>
      {children}
    </div>
  )
})

export default SpinLoading
