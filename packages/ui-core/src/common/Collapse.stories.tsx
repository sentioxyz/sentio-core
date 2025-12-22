import '../styles.css'
import { Collapse } from './Collapse'

// Ladle stories - export simple React components

export const Default = () => {
  return (
    <div className="p-4">
      <Collapse title="Click to expand">
        <div className="rounded-md border p-4">
          <p className="text-sm">
            This is the content inside the collapsible panel.
          </p>
          <p className="mt-2 text-sm">It can contain any React elements.</p>
        </div>
      </Collapse>
    </div>
  )
}

export const DefaultOpen = () => {
  return (
    <div className="p-4">
      <Collapse title="Already expanded" defaultOpen={true}>
        <div className="rounded-md border bg-blue-50 p-4">
          <p className="text-sm">This panel is open by default.</p>
          <p className="mt-2 text-sm">
            Set{' '}
            <code className="rounded bg-gray-200 px-1 py-0.5">
              defaultOpen=true
            </code>{' '}
            to achieve this.
          </p>
        </div>
      </Collapse>
    </div>
  )
}

export const CustomTitle = () => {
  return (
    <div className="space-y-4 p-4">
      <Collapse
        title={
          <span className="text-primary-600 font-bold">
            Custom Styled Title
          </span>
        }
      >
        <div className="rounded-md border p-4">
          <p className="text-sm">
            The title can be a React node with custom styling.
          </p>
        </div>
      </Collapse>

      <Collapse
        title={
          <span className="flex items-center gap-2">
            <span className="text-lg">📝</span>
            <span>Title with Icon</span>
          </span>
        }
      >
        <div className="rounded-md border p-4">
          <p className="text-sm">
            You can include icons or other elements in the title.
          </p>
        </div>
      </Collapse>
    </div>
  )
}

export const WithCustomClass = () => {
  return (
    <div className="p-4">
      <Collapse
        title="Panel with custom classes"
        className="border-primary-500 bg-primary-50 rounded-lg border-2 p-2"
        titleClassName="text-primary-700 font-semibold"
      >
        <div className="p-4">
          <p className="text-sm">
            This panel has custom className and titleClassName applied.
          </p>
        </div>
      </Collapse>
    </div>
  )
}

export const CustomIconSize = () => {
  return (
    <div className="space-y-4 p-4">
      <Collapse title="Small Icon" iconClassName="h-4 w-4">
        <div className="rounded-md border p-4">
          <p className="text-sm">Smaller chevron icon (h-4 w-4)</p>
        </div>
      </Collapse>

      <Collapse title="Large Icon" iconClassName="h-8 w-8">
        <div className="rounded-md border p-4">
          <p className="text-sm">Larger chevron icon (h-8 w-8)</p>
        </div>
      </Collapse>
    </div>
  )
}

export const LongContent = () => {
  return (
    <div className="p-4">
      <Collapse title="Expand to see long content">
        <div className="space-y-2 rounded-md border p-4">
          {Array.from({ length: 20 }).map((_, i) => (
            <p key={i} className="text-sm">
              Lorem ipsum dolor sit amet, consectetur adipiscing elit. Paragraph{' '}
              {i + 1}.
            </p>
          ))}
        </div>
      </Collapse>
    </div>
  )
}

export const NestedCollapse = () => {
  return (
    <div className="p-4">
      <Collapse title="Parent Panel">
        <div className="space-y-4 rounded-md border p-4">
          <p className="text-sm">This is the parent content.</p>

          <Collapse title="Nested Panel 1">
            <div className="rounded-md border bg-gray-50 p-4">
              <p className="text-sm">First nested panel content.</p>
            </div>
          </Collapse>

          <Collapse title="Nested Panel 2">
            <div className="rounded-md border bg-gray-50 p-4">
              <p className="text-sm">Second nested panel content.</p>
            </div>
          </Collapse>
        </div>
      </Collapse>
    </div>
  )
}

export const MultipleCollapse = () => {
  return (
    <div className="space-y-4 p-4">
      <Collapse title="Section 1: Introduction">
        <div className="rounded-md border p-4">
          <h3 className="mb-2 font-semibold">Introduction</h3>
          <p className="text-sm">
            This is the first section with introductory content.
          </p>
        </div>
      </Collapse>

      <Collapse title="Section 2: Details">
        <div className="rounded-md border p-4">
          <h3 className="mb-2 font-semibold">Details</h3>
          <p className="text-sm">This section contains detailed information.</p>
        </div>
      </Collapse>

      <Collapse title="Section 3: Conclusion">
        <div className="rounded-md border p-4">
          <h3 className="mb-2 font-semibold">Conclusion</h3>
          <p className="text-sm">Final thoughts and conclusions.</p>
        </div>
      </Collapse>
    </div>
  )
}

export const DarkMode = () => {
  return (
    <div className="bg-sentio-gray-50 dark min-h-screen p-4">
      <Collapse title="Dark Mode Panel" defaultOpen={true}>
        <div className="rounded-md border border-gray-700 bg-gray-800 p-4">
          <p className="text-sm text-gray-300">
            This panel looks good in dark mode.
          </p>
          <p className="mt-2 text-sm text-gray-300">
            The collapse component adapts to dark mode styling.
          </p>
        </div>
      </Collapse>
    </div>
  )
}

export const InteractiveContent = () => {
  return (
    <div className="p-4">
      <Collapse title="Interactive Content">
        <div className="space-y-4 rounded-md border p-4">
          <div>
            <label className="mb-2 block text-sm font-medium">
              Enter your name:
            </label>
            <input
              type="text"
              placeholder="Your name"
              className="w-full rounded border px-3 py-2"
            />
          </div>

          <div>
            <label className="mb-2 block text-sm font-medium">
              Choose an option:
            </label>
            <select className="w-full rounded border px-3 py-2">
              <option>Option 1</option>
              <option>Option 2</option>
              <option>Option 3</option>
            </select>
          </div>

          <button className="bg-primary-600 hover:bg-primary-700 rounded px-4 py-2 text-white">
            Submit
          </button>
        </div>
      </Collapse>
    </div>
  )
}
