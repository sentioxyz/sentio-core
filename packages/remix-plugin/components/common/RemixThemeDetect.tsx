'use client'

import { useEffect } from 'react'

export const RemixThemeDetect = () => {
  useEffect(() => {
    const element = document.querySelector('html')
    var observer = new MutationObserver(function (mutations) {
      mutations.forEach(function (mutation) {
        if (mutation.type === 'attributes') {
          console.log('attributes changed')
          const target = mutation.target as HTMLElement
          const theme = target.style.getPropertyValue('--theme')
          if (theme === 'dark') {
            document.querySelector('body')?.classList.add('dark')
          } else {
            document.querySelector('body')?.classList.remove('dark')
          }
        }
      })
    })

    if (element) {
      observer.observe(element, {
        attributes: true //configure it to listen to attribute changes
      })
    }
  }, [])

  return null
}
