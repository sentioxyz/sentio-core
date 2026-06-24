import { useDarkMode } from '../../utils/use-dark-mode'

export const ScatterIcon = () => {
  const isDarkMode = useDarkMode()
  const primary = isDarkMode ? '#B14598' : '#F36AD9'
  const bg = isDarkMode ? '#0B0714' : 'white'

  return (
    <svg
      width="56"
      height="56"
      viewBox="229 512 56 56"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M249 528C252.314 528 255 530.686 255 534C255 537.314 252.314 540 249 540C245.686 540 243 537.314 243 534C243 530.686 245.686 528 249 528Z"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
      />
      <path
        d="M271 543C272.657 543 274 544.343 274 546C274 547.657 272.657 549 271 549C269.343 549 268 547.657 268 546C268 544.343 269.343 543 271 543Z"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
      />
      <path
        d="M271 523C273.761 523 276 525.239 276 528C276 530.761 273.761 533 271 533C268.239 533 266 530.761 266 528C266 525.239 268.239 523 271 523Z"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
      />
      <path
        d="M261 547C263.209 547 265 548.791 265 551C265 553.209 263.209 555 261 555C258.791 555 257 553.209 257 551C257 548.791 258.791 547 261 547Z"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
      />
      <path
        d="M262 534C263.657 534 265 535.343 265 537C265 538.657 263.657 540 262 540C260.343 540 259 538.657 259 537C259 535.343 260.343 534 262 534Z"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
      />
      <path
        d="M253 543C254.105 543 255 543.895 255 545C255 546.105 254.105 547 253 547C251.895 547 251 546.105 251 545C251 543.895 251.895 543 253 543Z"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
      />
      <path
        d="M244 549C245.657 549 247 550.343 247 552C247 553.657 245.657 555 244 555C242.343 555 241 553.657 241 552C241 550.343 242.343 549 244 549Z"
        fill={bg}
        stroke={primary}
        strokeWidth="2"
      />
      <path
        d="M236 518C235.448 518 235 518.448 235 519V561C235 561.552 235.448 562 236 562L278 562C278.552 562 279 561.552 279 561C279 560.448 278.552 560 278 560L237 560V519C237 518.448 236.552 518 236 518Z"
        fill={primary}
      />
    </svg>
  )
}
