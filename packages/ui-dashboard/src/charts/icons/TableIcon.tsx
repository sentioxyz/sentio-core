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
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <g clipPath="url(#clip0_3670_4416)">
      <path
        d="M11.5 2H2.5C1.67157 2 1 2.55964 1 3.25V10.75C1 11.4404 1.67157 12 2.5 12H11.5C12.3284 12 13 11.4404 13 10.75V3.25C13 2.55964 12.3284 2 11.5 2Z"
        stroke="currentColor"
        strokeWidth="1.16667"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M1 5L13 5"
        stroke="currentColor"
        strokeWidth="1.16667"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M6 2L6 12"
        stroke="currentColor"
        strokeWidth="1.16667"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </g>
    <defs>
      <clipPath id="clip0_3670_4416">
        <rect width="14" height="14" fill="white" />
      </clipPath>
    </defs>
  </svg>
)

export default SvgIcon
