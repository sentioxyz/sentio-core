/**
 * Tracking utility for analytics events
 * This is a no-op implementation that can be overridden by the consuming application
 */

let trackingHandler:
  | ((eventName: string, properties?: Record<string, any>) => void)
  | null = null

/**
 * Sets the tracking handler for analytics events
 * @param handler Function to handle tracking events
 */
export function setTrackingHandler(
  handler: (eventName: string, properties?: Record<string, any>) => void
) {
  trackingHandler = handler
}

/**
 * Tracks an analytics event
 * @param eventName The name of the event to track
 * @param properties Optional properties to include with the event
 */
export function trackEvent(
  eventName: string,
  properties?: Record<string, any>
): void {
  if (trackingHandler) {
    trackingHandler(eventName, properties)
  }
}
