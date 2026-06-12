// Reuse the @sentio/ui-core theme verbatim so dashboard components share the
// exact same color tokens, button utilities and dark-mode setup. ui-dashboard
// does NOT redefine the theme — at runtime the CSS variables come from
// @sentio/ui-core's published style.css, which consumers must also import.
//
// This file is only consumed at build time (not published in `files`), so the
// relative require into the sibling workspace package is safe.
// eslint-disable-next-line @typescript-eslint/no-var-requires
module.exports = require('../ui-core/tailwind.config.js')
