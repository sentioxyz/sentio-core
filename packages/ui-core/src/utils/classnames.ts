import { cx } from 'class-variance-authority'
// import { twMerge } from 'tailwind-merge'
import type { ClassValue } from 'class-variance-authority/types'

export { cx as classNames }

export const mergeClasses = (...inputs: ClassValue[]) => cx(inputs)
