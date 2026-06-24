import { useDarkMode } from '../../utils/use-dark-mode'

export const RetentionIcon = () => {
  const isDarkMode = useDarkMode()
  const primary = isDarkMode ? '#1FD6D6' : '#18B5B5'
  const bg = isDarkMode ? '#0B0714' : 'white'

  return (
    <svg
      width="56"
      height="56"
      viewBox="73 1302 56 56"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M77 1307C77 1306.45 77.5117 1306 78.1429 1306H123.857C124.488 1306 125 1306.45 125 1307C125 1307.55 124.488 1308 123.857 1308H78.1429C77.5117 1308 77 1307.55 77 1307Z"
        fill={primary}
      />
      <rect
        x="78"
        y="1312"
        width="46"
        height="6"
        rx="1"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
      />
      <rect
        x="78"
        y="1323"
        width="8"
        height="8"
        rx="1"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
      />
      <rect
        x="92"
        y="1323"
        width="8"
        height="8"
        rx="1"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
      />
      <rect
        x="106"
        y="1323"
        width="8"
        height="8"
        rx="1"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
      />
      <rect
        x="92"
        y="1335"
        width="8"
        height="8"
        rx="1"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
      />
      <rect
        x="78"
        y="1335"
        width="8"
        height="8"
        rx="1"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
      />
      <rect
        x="78"
        y="1347"
        width="8"
        height="8"
        rx="1"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
      />
    </svg>
  )
}
