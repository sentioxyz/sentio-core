import { useDarkMode } from '../../utils/use-dark-mode'

export const GroupsIcon = () => {
  const isDarkMode = useDarkMode()
  if (isDarkMode) {
    return (
      <svg
        width="56"
        height="56"
        viewBox="0 0 56 56"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <rect
          x="4"
          y="20"
          width="30"
          height="12"
          rx="2"
          fill="#0B0714"
          stroke="#B14598"
          strokeWidth="2"
          strokeLinejoin="round"
        />
        <rect
          x="23"
          y="37"
          width="30"
          height="12"
          rx="2"
          fill="#0B0714"
          stroke="#B14598"
          strokeWidth="2"
          strokeLinejoin="round"
        />
        <rect
          x="39"
          y="20"
          width="14"
          height="12"
          rx="2"
          fill="#0B0714"
          stroke="#B14598"
          strokeWidth="2"
          strokeLinejoin="round"
        />
        <rect
          x="4"
          y="37"
          width="14"
          height="12"
          rx="2"
          fill="#0B0714"
          stroke="#B14598"
          strokeWidth="2"
          strokeLinejoin="round"
        />
        <rect x="11" y="8" width="42" height="4" rx="2" fill="#B14598" />
        <circle cx="5" cy="10" r="2" fill="#B14598" />
      </svg>
    )
  }

  return (
    <svg
      width="56"
      height="56"
      viewBox="0 0 56 56"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <rect
        x="4"
        y="20"
        width="30"
        height="12"
        rx="2"
        fill="white"
        stroke="#F36AD9"
        strokeWidth="2"
        strokeLinejoin="round"
      />
      <rect
        x="23"
        y="37"
        width="30"
        height="12"
        rx="2"
        fill="white"
        stroke="#F36AD9"
        strokeWidth="2"
        strokeLinejoin="round"
      />
      <rect
        x="39"
        y="20"
        width="14"
        height="12"
        rx="2"
        fill="white"
        stroke="#F36AD9"
        strokeWidth="2"
        strokeLinejoin="round"
      />
      <rect
        x="4"
        y="37"
        width="14"
        height="12"
        rx="2"
        fill="white"
        stroke="#F36AD9"
        strokeWidth="2"
        strokeLinejoin="round"
      />
      <rect x="11" y="8" width="42" height="4" rx="2" fill="#F36AD9" />
      <circle cx="5" cy="10" r="2" fill="#F36AD9" />
    </svg>
  )
}
