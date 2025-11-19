import type { Metadata } from 'next'
import Script from 'next/script'
import { Roboto } from 'next/font/google'
import './globals.css'
import { Toaster } from '@/components/ui/toaster'

const fontSans = Roboto({
  subsets: ['latin'],
  variable: '--font-sans',
  weight: '400'
})

export const metadata: Metadata = {
  title: 'Sentio Remix Plugin',
  description: 'Sentio plugin for Remix IDE, support function view, simulate and more.'
}

export default function RootLayout({
  children
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html lang="en">
      <Script src="https://kit.fontawesome.com/fe456af2e0.js" crossOrigin="anonymous" strategy="afterInteractive" />
      <body className={'bg-light min-h-screen font-sans antialiased ' + fontSans.className}>
        {children}
        <Toaster />
      </body>
    </html>
  )
}
