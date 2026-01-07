import '../styles.css'
import NewButton, { Proccessing as ProcessingIcon } from './NewButton'
import { LuHeart } from 'react-icons/lu'
import { useState } from 'react'

// Ladle works by rendering exported React components from *.stories.* files.
// Export simple React components (no Storybook APIs) so Ladle can pick them up.

export const Default = () => (
  <div style={{ padding: 16 }} className="space-y-4">
    <NewButton role="secondary" size="default">
      Button
    </NewButton>
    <div className="flex gap-2">
      <ProcessingIcon light={true} />
      <span>Processing...</span>
    </div>
  </div>
)

export const Text = () => (
  <div style={{ padding: 16 }}>
    <div>
      <NewButton role="text" size="md">
        Text Button
      </NewButton>
    </div>
    <div className="mt-4">
      <NewButton role="text" size="md" processing={true}>
        Text Button
      </NewButton>
    </div>
  </div>
)

export const Primary = () => (
  <div style={{ padding: 16 }}>
    <div>
      <NewButton role="primary" size="md">
        Primary
      </NewButton>
    </div>
    <div className="mt-4">
      <NewButton role="primary" size="md" processing={true}>
        Primary
      </NewButton>
    </div>
  </div>
)

export const AllRoles = () => {
  return (
    <div className="flex gap-4">
      {['primary', 'secondary', 'text', 'link', 'tertiary'].map((role) => (
        <NewButton key={role} role={role as any}>
          {role.charAt(0).toUpperCase() + role.slice(1)}
        </NewButton>
      ))}
    </div>
  )
}

export const WithIcon = () => {
  const [like, setLike] = useState(false)
  return (
    <div style={{ padding: 16 }}>
      <NewButton
        role={like ? 'primary' : 'secondary'}
        size="default"
        icon={<LuHeart className="h-4 w-4" />}
        onClick={() => setLike((v) => !v)}
      >
        Like
      </NewButton>
    </div>
  )
}

export const Processing = () => (
  <div style={{ padding: 16 }}>
    <NewButton role="primary" processing>
      Saving
    </NewButton>
  </div>
)

export const DisabledWithHint = () => (
  <div style={{ padding: 16 }}>
    <NewButton
      role="secondary"
      disabled
      disabledHint="You don’t have permission"
    >
      Disabled
    </NewButton>
  </div>
)
