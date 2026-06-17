import React from 'react'

interface Props {
  className?: string
}
const SvgIcon = ({ className }: Props) => (
  <svg
    width="14"
    height="14"
    viewBox="0 0 14 14"
    fill="none"
    className={className}
    xmlns="http://www.w3.org/2000/svg"
  >
    <g clipPath="url(#clip0_3670_4424)">
      <path
        d="M11.5 1.5H2.5C1.67157 1.5 1 2.11561 1 2.875V11.125C1 11.8844 1.67157 12.5 2.5 12.5H11.5C12.3284 12.5 13 11.8844 13 11.125V2.875C13 2.11561 12.3284 1.5 11.5 1.5Z"
        stroke="currentColor"
        strokeWidth="1.16667"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M7.98188 5.77273H6.39097L5.75461 5.13636L6.39097 4.5H7.98188L8.61824 5.13636L7.98188 5.77273Z"
        fill="currentColor"
      />
      <path
        d="M7.98188 11.5H6.39097L5.75461 10.8637L6.39097 10.2273H7.98188L8.61824 10.8637L7.98188 11.5Z"
        fill="currentColor"
      />
      <path
        d="M8.30005 9.90907V6.09089L8.93641 5.45453L9.57278 6.09089V9.90907L8.93641 10.5454L8.30005 9.90907Z"
        fill="currentColor"
      />
      <path
        d="M4.80005 9.90907V6.09089L5.43641 5.45453L6.07278 6.09089V9.90907L5.43641 10.5454L4.80005 9.90907Z"
        fill="currentColor"
      />
      <path
        d="M1 3.5L13 3.5"
        stroke="currentColor"
        strokeWidth="1.16667"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </g>
    <defs>
      <clipPath id="clip0_3670_4424">
        <rect width="14" height="14" fill="white" />
      </clipPath>
    </defs>
  </svg>
)

export default SvgIcon
