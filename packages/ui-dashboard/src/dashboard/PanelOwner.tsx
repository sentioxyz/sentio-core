import { PopoverTooltip } from '@sentio/ui-core'
import { ImgWithFallback } from '../common/ImgWithFallback'
import type { UserInfoLike } from '../types'

interface Props {
  creator?: UserInfoLike
  updater?: UserInfoLike
  /** Fallback project-owner display (used when neither creator nor updater is set). */
  ownerName?: string
  ownerAvatarUrl?: string
  /** When provided, tooltip user names become clickable (dash mode). */
  onNavigateToUser?: (username: string) => void
}

const USER_CONTAINER_CLASSES =
  'inline-flex items-center gap-1 px-1 text-xs cursor-pointer'
const AVATAR_CLASSES = 'h-3 w-3 rounded-full'
const TOOLTIP_CLASSES = 'flex items-center gap-2'
const LABEL_CLASSES = 'text-xs font-semibold text-text-foreground-secondary'

function displayName(user: UserInfoLike) {
  return user.firstName && user.lastName
    ? `${user.firstName} ${user.lastName}`
    : user.username
}

export const UserInfo = ({
  avatarSrc,
  avatarAlt,
  username,
  containerClassName = USER_CONTAINER_CLASSES
}: {
  avatarSrc?: string
  avatarAlt?: string
  username?: string
  containerClassName?: string
}) => (
  <div className={containerClassName}>
    {avatarSrc && (
      <ImgWithFallback
        className={AVATAR_CLASSES}
        src={avatarSrc}
        alt={avatarAlt || username || ''}
      />
    )}
    <span className="max-w-[120px] truncate">{username}</span>
  </div>
)

const TooltipUserDisplay = ({
  user,
  label,
  onNavigateToUser
}: {
  user: UserInfoLike
  label: string
  onNavigateToUser?: (username: string) => void
}) => (
  <div className="flex flex-col gap-1 p-1">
    <div className={LABEL_CLASSES}>{label}</div>
    <div className={TOOLTIP_CLASSES}>
      <ImgWithFallback
        className="h-4 w-4 rounded-full"
        src={user.picture}
        alt={user.username}
      />
      {onNavigateToUser && user.username ? (
        <a
          className="text-xs font-medium hover:underline"
          href={`/user/${user.username}`}
          onClick={(e) => {
            e.preventDefault()
            onNavigateToUser(user.username!)
          }}
        >
          {displayName(user)}
        </a>
      ) : (
        <span className="text-xs font-medium">{displayName(user)}</span>
      )}
    </div>
  </div>
)

const UserWithTooltip = ({
  mainUser,
  tooltipUser,
  tooltipLabel,
  onNavigateToUser
}: {
  mainUser: UserInfoLike
  tooltipUser: UserInfoLike
  tooltipLabel: string
  onNavigateToUser?: (username: string) => void
}) => (
  <PopoverTooltip
    text={
      <TooltipUserDisplay
        user={tooltipUser}
        label={tooltipLabel}
        onNavigateToUser={onNavigateToUser}
      />
    }
    maxWidth="max-w-[240px]"
    offsetOptions={10}
  >
    <UserInfo
      avatarSrc={mainUser.picture}
      avatarAlt={mainUser.username}
      username={mainUser.username}
    />
  </PopoverTooltip>
)

const UserWithBothTooltip = ({
  mainUser,
  creator,
  updater,
  onNavigateToUser
}: {
  mainUser: UserInfoLike
  creator: UserInfoLike
  updater: UserInfoLike
  onNavigateToUser?: (username: string) => void
}) => (
  <PopoverTooltip
    text={
      <div className="space-y-3 p-1">
        <TooltipUserDisplay
          user={creator}
          label="Created by"
          onNavigateToUser={onNavigateToUser}
        />
        <TooltipUserDisplay
          user={updater}
          label="Last updated by"
          onNavigateToUser={onNavigateToUser}
        />
      </div>
    }
    maxWidth="max-w-[280px]"
    offsetOptions={10}
  >
    <UserInfo
      avatarSrc={mainUser.picture}
      avatarAlt={mainUser.username}
      username={mainUser.username}
    />
  </PopoverTooltip>
)

export const PanelOwner = ({
  creator,
  updater,
  ownerName,
  ownerAvatarUrl,
  onNavigateToUser
}: Props) => {
  if (!creator && !updater) {
    return (
      <UserInfo
        avatarSrc={ownerAvatarUrl}
        avatarAlt={ownerName}
        username={ownerName}
      />
    )
  }

  if (creator && updater) {
    return (
      <UserWithBothTooltip
        mainUser={updater}
        creator={creator}
        updater={updater}
        onNavigateToUser={onNavigateToUser}
      />
    )
  }

  if (creator) {
    return (
      <UserWithTooltip
        mainUser={creator}
        tooltipUser={creator}
        tooltipLabel="Created by"
        onNavigateToUser={onNavigateToUser}
      />
    )
  }

  if (updater) {
    return (
      <UserWithTooltip
        mainUser={updater}
        tooltipUser={updater}
        tooltipLabel="Last updated by"
        onNavigateToUser={onNavigateToUser}
      />
    )
  }

  return null
}
