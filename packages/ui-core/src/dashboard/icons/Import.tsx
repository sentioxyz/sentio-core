import { useDarkMode } from '../../utils/use-dark-mode'

export const ImportIcon = ({ className }: { className?: string }) => {
  const isDarkMode = useDarkMode()

  const stroke = isDarkMode ? '#B14598' : '#F36AD9'

  return (
    <svg
      width="56"
      height="56"
      viewBox="0 0 56 56"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M15.1762 11L4 31.8889V48C4 50.2091 5.79086 52 8 52H48C50.2091 52 52 50.2091 52 48V31.8889L40.8238 11H15.1762Z"
        stroke={stroke}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M4 32L19.0941 32L24.1765 39H31.8235L36.9059 32L52 32"
        stroke={stroke}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M28 3V26"
        stroke={stroke}
        strokeWidth="2"
        strokeLinecap="round"
      />
      <path
        d="M19 18L28 27L37 18"
        stroke={stroke}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}
