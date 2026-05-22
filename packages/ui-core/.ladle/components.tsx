import { useEffect } from 'react'
import { useLadleContext, ThemeState, type GlobalProvider } from '@ladle/react'

/**
 * Ladle's default dark mode toggle adds `data-theme="dark"` on <html>.
 * Our Tailwind config uses `darkMode: 'selector'` (`.dark` class) and our
 * theme-variables.css scopes dark tokens to `body.dark`. This provider
 * mirrors Ladle's theme state onto the <body> element so the existing
 * `body.dark` rules light up inside Ladle.
 */
export const Provider: GlobalProvider = ({ children }) => {
  const { globalState } = useLadleContext()
  const isDark = globalState.theme === ThemeState.Dark

  useEffect(() => {
    document.body.classList.toggle('dark', isDark)
    return () => {
      document.body.classList.remove('dark')
    }
  }, [isDark])

  return <>{children}</>
}
