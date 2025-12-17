'use client'

import { useState, useEffect } from 'react'

/**
 * React Hook for detecting mobile devices
 * @param breakpoint Breakpoint width, default 768px
 * @returns boolean Whether it's a mobile device
 */
export function useMobile(breakpoint: number = 768, defaultValue: boolean = false): boolean {
  const [isMobile, setIsMobile] = useState<boolean>(defaultValue)

  useEffect(() => {
    // Check user agent string
    const checkUserAgent = (): boolean => {
      if (typeof window === 'undefined') return false

      const userAgent = window.navigator.userAgent.toLowerCase()
      const mobileKeywords = [
        'android',
        'iphone',
        'ipad',
        'ipod',
        'blackberry',
        'windows phone',
        'mobile',
        'webos',
        'opera mini'
      ]

      return mobileKeywords.some((keyword) => userAgent.includes(keyword))
    }

    // Check screen width
    const checkScreenWidth = (): boolean => {
      if (typeof window === 'undefined') return false
      return window.innerWidth < breakpoint
    }

    // Check touch support
    const checkTouchSupport = (): boolean => {
      if (typeof window === 'undefined') return false
      return 'ontouchstart' in window || navigator.maxTouchPoints > 0
    }

    // Comprehensive mobile detection
    const detectMobile = (): boolean => {
      const isUserAgentMobile = checkUserAgent()
      const isScreenSmall = checkScreenWidth()
      const hasTouchSupport = checkTouchSupport()

      // If user agent explicitly indicates mobile device, return true directly
      if (isUserAgentMobile) return true

      // If screen is small and supports touch, consider it mobile
      if (isScreenSmall && hasTouchSupport) return true

      // Judge based on screen width only
      return isScreenSmall
    }

    // Initial detection
    setIsMobile(detectMobile())

    // Listen for window resize
    const handleResize = () => {
      setIsMobile(detectMobile())
    }

    window.addEventListener('resize', handleResize)

    // Cleanup event listener
    return () => {
      window.removeEventListener('resize', handleResize)
    }
  }, [breakpoint])

  return isMobile
}

export function useInDapp() {
  const isMobile = useMobile()
  if (process.env.NODE_ENV === 'development') {
    return isMobile
  }

  // @ts-ignore: Ignore missing type for isBinance
  const isBinanceBrowser = typeof window === 'object' && window.ethereum?.isBinance

  return isBinanceBrowser
}
