// eslint-disable-next-line no-undef
const uiCoreConfig = require('@sentio/ui-core/tailwind.config.js')

// eslint-disable-next-line no-undef
module.exports = {
  ...uiCoreConfig,
  content: ['./out/**/*.{js,ts,jsx,tsx}'],
  important: true
}
