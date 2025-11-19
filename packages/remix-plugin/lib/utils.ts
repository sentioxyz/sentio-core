import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function trimTxnHash(str?: string) {
  if (!str) {
    return str
  }
  return str.slice(0, 10) + '...' + str.slice(-4)
}

export function getMethodSignature(data?: string) {
  if (!data) {
    return data
  }
  return data.slice(0, 10)
}

export function uuid() {
  return Math.random().toString(16).slice(2)
}
