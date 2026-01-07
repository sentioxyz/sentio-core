import { useEffect, useState } from 'react'

class DarkModeListener {
  private static _instance: DarkModeListener
  private isDarkMode = false
  private listeners: ((isDarkMode: boolean) => void)[] = []

  static get instance() {
    if (!this._instance) {
      this._instance = new DarkModeListener()
    }
    return this._instance
  }

  constructor() {
    this.init()
  }

  public addListener(listener: (isDarkMode: boolean) => void) {
    this.listeners.push(listener)
  }

  public removeListener(listener: (isDarkMode: boolean) => void) {
    this.listeners = this.listeners.filter((l) => l !== listener)
  }

  public get darkMode() {
    return this.isDarkMode
  }

  private _sync(theme: 'light' | 'dark' | 'system' = 'system') {
    let isDarkMode = false
    if (theme === 'system') {
      const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
      isDarkMode = mediaQuery.matches
      localStorage.setItem('theme', 'system')
    } else if (theme === 'light') {
      isDarkMode = false
      localStorage.removeItem('theme')
    } else {
      isDarkMode = theme === 'dark'
      localStorage.setItem('theme', 'dark')
    }

    this.isDarkMode = isDarkMode
    document.body.classList.remove('light', 'dark')
    document.body.classList.add(isDarkMode ? 'dark' : 'light')
    this.listeners.forEach((listener) => listener(isDarkMode))
  }

  public toggleDarkMode() {
    this.isDarkMode = document.body.classList.contains('dark')
    this._sync(this.isDarkMode ? 'light' : 'dark')
  }

  public setDarkMode(value: 'light' | 'dark' | 'system') {
    this._sync(value)
  }

  private init() {
    this.isDarkMode = document.body.classList.contains('dark')
    // Create a MutationObserver to observe changes in the class attribute
    const observer = new MutationObserver((mutationsList) => {
      for (const mutation of mutationsList) {
        if (
          mutation.type === 'attributes' &&
          mutation.attributeName === 'class'
        ) {
          const isDarkMode = document.body.classList.contains('dark')
          if (this.isDarkMode !== isDarkMode) {
            this.isDarkMode = isDarkMode
            this.listeners.forEach((listener) => listener(isDarkMode))
          }
        }
      }
    })

    // Configure the observer to watch for attribute changes
    const config = {
      attributes: true, // Observe attribute changes
      attributeFilter: ['class'] // Only observe changes to the 'class' attribute
    }

    // Start observing the body element
    observer.observe(document.body, config)
  }
}

export const useDarkMode = () => {
  const [isDarkMode, setIsDarkMode] = useState(false)
  useEffect(() => {
    const instance = DarkModeListener.instance
    setIsDarkMode(instance.darkMode)
    instance.addListener(setIsDarkMode)
    return () => {
      instance.removeListener(setIsDarkMode)
    }
  }, [])

  return isDarkMode
}
