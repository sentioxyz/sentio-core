import type { Story } from '@ladle/react'
import { PanelOwner } from './PanelOwner'
import type { UserInfoLike } from '../types'

const alice: UserInfoLike = {
  username: 'alice',
  firstName: 'Alice',
  lastName: 'Lin',
  picture: ''
}
const bob: UserInfoLike = {
  username: 'bob',
  firstName: 'Bob',
  lastName: 'Wu',
  picture: ''
}

export const ProjectOwner: Story = () => (
  <div className="p-8">
    <PanelOwner ownerName="acme" ownerAvatarUrl="" />
  </div>
)

export const CreatorOnly: Story = () => (
  <div className="p-8">
    <PanelOwner
      creator={alice}
      onNavigateToUser={(u) => console.log('go', u)}
    />
  </div>
)

export const CreatorAndUpdater: Story = () => (
  <div className="p-8">
    <PanelOwner
      creator={alice}
      updater={bob}
      onNavigateToUser={(u) => console.log('go', u)}
    />
  </div>
)
