import { useDarkMode } from '../../utils/use-dark-mode'

export const SQlIcon = () => {
  const isDarkMode = useDarkMode()
  const primary = isDarkMode ? '#F28C38' : '#EE8B27'
  const bg = isDarkMode ? '#0B0714' : 'white'

  return (
    <svg
      width="56"
      height="56"
      viewBox="73 1460 56 56"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M121 1470H81C80.4477 1470 80 1470.06 80 1470.14V1474.86C80 1474.94 80.4477 1475 81 1475H121C121.552 1475 122 1474.94 122 1474.86V1470.14C122 1470.06 121.552 1470 121 1470Z"
        fill={bg}
      />
      <path
        d="M98 1483C98 1482.45 98.4477 1482 99 1482H119C119.552 1482 120 1482.45 120 1483C120 1483.55 119.552 1484 119 1484H99C98.4477 1484 98 1483.55 98 1483Z"
        fill={primary}
      />
      <path
        d="M82 1483C82 1482.45 82.4477 1482 83 1482H88C88.5523 1482 89 1482.45 89 1483C89 1483.55 88.5523 1484 88 1484H83C82.4477 1484 82 1483.55 82 1483Z"
        fill={primary}
      />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M81 1468H121C122.657 1468 124 1469.34 124 1471V1505C124 1506.66 122.657 1508 121 1508H81C79.3432 1508 78 1506.66 78 1505V1471C78 1469.34 79.3431 1468 81 1468ZM81 1470H121C121.552 1470 122 1470.45 122 1471V1505C122 1505.55 121.552 1506 121 1506H81C80.4477 1506 80 1505.55 80 1505V1471C80 1470.45 80.4477 1470 81 1470Z"
        fill={primary}
      />
      <path d="M80 1491H122V1506H80V1491Z" fill={bg} />
      <path
        d="M83 1501.5L87 1498.5L83 1495.5"
        stroke={primary}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M90 1498.5H93"
        stroke={primary}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M98 1498.5C98 1497.95 98.4477 1497.5 99 1497.5H119C119.552 1497.5 120 1497.95 120 1498.5C120 1499.05 119.552 1499.5 119 1499.5H99C98.4477 1499.5 98 1499.05 98 1498.5Z"
        fill={primary}
      />
      <path d="M80 1475H122V1477H80V1475Z" fill={primary} />
      <path d="M80 1489H122V1491H80V1489Z" fill={primary} />
    </svg>
  )
}
