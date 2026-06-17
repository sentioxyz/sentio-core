// Plain (non-hook) UA check — mirror of the app's `lib/mobile`. Used by
// EchartsBase inside an effect, where a hook can't be called.
export function isMobile() {
  return /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(
    navigator.userAgent
  )
}
