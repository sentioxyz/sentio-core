// ui-dashboard components rely on @sentio/ui-core for theme tokens, the base
// input/select styling and shared utilities. ui-dashboard's own style.css only
// emits its extra utilities (tokens come from ui-core at runtime). Mirror the
// real consumer setup by loading both built stylesheets — run `pnpm build`
// (or `pnpm dev:css` in both packages) so the dist CSS is fresh.
import '@sentio/ui-core/dist/style.css'
import '../dist/style.css'

import { useEffect } from 'react'
import { useLadleContext, ThemeState, type GlobalProvider } from '@ladle/react'

/**
 * Mirror Ladle's theme state onto <body> so ui-core's `body.dark` token
 * overrides light up (same pattern as the ui-core Ladle provider).
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
