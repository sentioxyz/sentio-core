import '../styles.css'
import { DisclosurePanel } from './DisclosurePanel'

// Ladle stories - export simple React components

export const Default = () => {
  return (
    <div className="p-4">
      <DisclosurePanel title="Click to expand">
        <div className="space-y-2">
          <p className="text-sm">
            This is the content inside the disclosure panel.
          </p>
          <p className="text-sm">It can contain any React elements.</p>
        </div>
      </DisclosurePanel>
    </div>
  )
}

export const DefaultOpen = () => {
  return (
    <div className="p-4">
      <DisclosurePanel title="Already expanded" defaultOpen={true}>
        <div className="space-y-2">
          <p className="text-sm">This panel is open by default.</p>
          <p className="text-sm">
            Set{' '}
            <code className="rounded bg-gray-200 px-1 py-0.5 text-xs">
              defaultOpen=true
            </code>{' '}
            to achieve this.
          </p>
        </div>
      </DisclosurePanel>
    </div>
  )
}

export const DynamicTitle = () => {
  return (
    <div className="p-4">
      <DisclosurePanel
        title={(open) => (
          <span className="font-medium">
            {open ? '▼ Click to collapse' : '▶ Click to expand'}
          </span>
        )}
      >
        <div className="space-y-2">
          <p className="text-sm">
            The title can be a function that receives the open state.
          </p>
          <p className="text-sm">
            This allows you to dynamically change the title based on whether the
            panel is open or closed.
          </p>
        </div>
      </DisclosurePanel>
    </div>
  )
}

export const CustomStyling = () => {
  return (
    <div className="space-y-4 p-4">
      <DisclosurePanel
        title="Custom Container Style"
        containerClassName="rounded-lg border-2 border-blue-300 bg-blue-50"
        titleClassName="text-blue-700 hover:bg-blue-100"
        className="bg-blue-100"
      >
        <div className="space-y-2">
          <p className="text-sm">Custom styled panel with blue theme.</p>
          <p className="text-sm">
            Use{' '}
            <code className="rounded bg-white px-1 py-0.5 text-xs">
              containerClassName
            </code>
            ,{' '}
            <code className="rounded bg-white px-1 py-0.5 text-xs">
              titleClassName
            </code>
            , and{' '}
            <code className="rounded bg-white px-1 py-0.5 text-xs">
              className
            </code>{' '}
            for styling.
          </p>
        </div>
      </DisclosurePanel>

      <DisclosurePanel
        title="Another Custom Style"
        containerClassName="rounded-lg border-2 border-green-300 bg-green-50"
        titleClassName="text-green-700 hover:bg-green-100"
        iconClassName="h-4 w-4"
      >
        <div className="space-y-2">
          <p className="text-sm">Custom styled panel with green theme.</p>
          <p className="text-sm">
            The icon size is also customizable via{' '}
            <code className="rounded bg-white px-1 py-0.5 text-xs">
              iconClassName
            </code>
            .
          </p>
        </div>
      </DisclosurePanel>
    </div>
  )
}

export const MultipleNested = () => {
  return (
    <div className="space-y-4 p-4">
      <DisclosurePanel title="Parent Panel" defaultOpen={true}>
        <div className="space-y-3">
          <p className="text-sm">This is the parent panel content.</p>

          <DisclosurePanel title="Nested Panel 1">
            <p className="text-sm">This is nested content inside Panel 1.</p>
          </DisclosurePanel>

          <DisclosurePanel title="Nested Panel 2">
            <div className="space-y-2">
              <p className="text-sm">This is nested content inside Panel 2.</p>

              <DisclosurePanel title="Deeply Nested Panel">
                <p className="text-sm">
                  You can nest panels as deep as needed!
                </p>
              </DisclosurePanel>
            </div>
          </DisclosurePanel>
        </div>
      </DisclosurePanel>
    </div>
  )
}

export const DarkMode = () => {
  return (
    <div className="bg-sentio-gray-50 dark min-h-screen p-4">
      <div className="space-y-4">
        <DisclosurePanel title="Dark Mode Panel">
          <p className="text-sm text-gray-300">
            This panel uses the default dark mode styling.
          </p>
        </DisclosurePanel>

        <DisclosurePanel title="Another Dark Panel" defaultOpen={true}>
          <div className="space-y-2">
            <p className="text-sm text-gray-300">
              The dark mode is automatically applied based on the dark class.
            </p>
            <p className="text-sm text-gray-400">
              All interactive elements adapt to the dark theme.
            </p>
          </div>
        </DisclosurePanel>
      </div>
    </div>
  )
}
