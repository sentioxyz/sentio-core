import { useEffect, useState } from 'react'
import { classNames } from '@sentio/ui-core'

interface Props extends React.ImgHTMLAttributes<HTMLImageElement> {
  fallback?: string
}

const TRANSPARENT_GIF =
  'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAICRAEAOw=='

export const ImgWithFallback = ({
  src,
  fallback = TRANSPARENT_GIF,
  ...props
}: Props) => {
  const [loadError, setLoadError] = useState(false)

  useEffect(() => {
    // reset loadError when src changes
    setLoadError(false)
  }, [src])

  return (
    <img
      alt=""
      data-src={src}
      src={loadError ? fallback : (src ?? fallback)}
      {...props}
      className={classNames(
        props.className,
        loadError && 'bg-gray-200 dark:bg-gray-700'
      )}
      onError={() => {
        setLoadError(true)
      }}
    />
  )
}
