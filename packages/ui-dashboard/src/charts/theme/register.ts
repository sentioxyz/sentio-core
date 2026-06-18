import { registerTheme } from 'echarts/core'
import { sentioTheme, sentioThemeDark } from './sentio-theme'

// Registers the 'sentio' / 'sentio-dark' ECharts themes. Exposed as a function
// (not a bare side-effect import) so it survives tree-shaking: ui-dashboard's
// package.json marks only *.css as side-effectful, so a side-effect-only
// `import './register'` would be dropped by the bundler and the themes would
// never register (charts fall back to ECharts' default palette). Callers invoke
// this before `echarts.init(node, 'sentio')`. Idempotent.
let registered = false
export function registerSentioTheme() {
  if (registered) return
  registered = true
  registerTheme('sentio', sentioTheme)
  registerTheme('sentio-dark', sentioThemeDark)
}
