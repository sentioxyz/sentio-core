export function checkInjectElementAndLog(element: any, message?: string) {
  if (!element || element.length === 0) {
    // mixpanel.track('Extension component cannot inject', {
    //   location: document.location.href,
    //   message: message
    // })
  }
  return element
}
