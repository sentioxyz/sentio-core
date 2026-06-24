import { useDarkMode } from '../../utils/use-dark-mode'

export const EventLogsIcon = () => {
  const isDarkMode = useDarkMode()
  const primary = isDarkMode ? '#5B7CFF' : '#2E4EEB'
  const bg = isDarkMode ? '#0B0714' : 'white'

  return (
    <svg
      width="56"
      height="56"
      viewBox="71 1146 56 56"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M77 1153H120V1196H89.2857L83.1429 1189.86L77 1183.71V1153Z"
        fill={bg}
      />
      <path
        d="M104.25 1188.25C107.564 1188.25 110.25 1185.56 110.25 1182.25C110.25 1178.94 107.564 1176.25 104.25 1176.25C100.936 1176.25 98.25 1178.94 98.25 1182.25C98.25 1185.56 100.936 1188.25 104.25 1188.25Z"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M111.751 1189.74L108.488 1186.48"
        stroke={primary}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path d="M87.5 1185.5L79 1185H78L87.5 1194.5V1185.5Z" fill={bg} />
      <rect x="85" y="1160" width="8" height="2" rx="1" fill={primary} />
      <rect x="85" y="1168" width="28" height="2" rx="1" fill={primary} />
      <rect x="85" y="1176" width="10" height="2" rx="1" fill={primary} />
      <rect x="95" y="1160" width="8" height="2" rx="1" fill={primary} />
      <rect x="105" y="1160" width="8" height="2" rx="1" fill={primary} />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M79 1151H119V1153H79C78.4477 1153 78 1153.45 78 1154V1184H85C87.2091 1184 89 1185.79 89 1188V1195H119C119.552 1195 120 1194.55 120 1194V1154C120 1153.45 119.552 1153 119 1153V1151C120.657 1151 122 1152.34 122 1154V1194C122 1195.66 120.657 1197 119 1197H88L76 1185V1154C76 1152.34 77.3431 1151 79 1151ZM79.8284 1186H85C86.1046 1186 87 1186.9 87 1188V1193.17L79.8284 1186Z"
        fill={primary}
      />
    </svg>
  )
}
